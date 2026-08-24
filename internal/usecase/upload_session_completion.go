package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"sort"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	domainnode "omniflow-go/internal/domain/node"
	domainsession "omniflow-go/internal/domain/uploadsession"
	"omniflow-go/internal/repository"
	"omniflow-go/internal/storage"
)

// 完成回执只承担网络不确定期内的结果重放；7 天后由同一个 janitor 清理。
const uploadCompletionReceiptTTL = 7 * 24 * time.Hour

// complete 认领必须跨过 janitor 的扫描窗口，避免临界过期时并发清理对象。
const uploadCompletionClaimMinTTL = 15 * time.Minute

const maxUploadClientOperationIDLength = 128

// CompleteUploadSessionCommand 完成上传所需参数。
// Parts 仅 multipart 模式必填；single 模式忽略。
// ConflictPolicy 在 Complete 阶段决定同名节点处理策略；空串走 NodeUseCase.Create 默认（error）。
type CompleteUploadSessionCommand struct {
	Actor             actor.Actor
	UploadID          string
	ClientOperationID string
	Parts             []CompletedPart
	ConflictPolicy    NodeNameConflictPolicy
}

const (
	UploadCompletionStateUnknown     = "unknown"
	UploadCompletionStateUncommitted = "uncommitted"
	UploadCompletionStateCommitted   = "committed"
)

// UploadCompletionStatusResult 是 complete 结果核对契约。Unknown 对未命中和其他 actor 的操作保持一致。
type UploadCompletionStatusResult struct {
	State string           `json:"state"`
	Node  *domainnode.Node `json:"node,omitempty"`
}

// CompletedPart 客户端在 complete 阶段提交的分片清单条目。
type CompletedPart struct {
	PartNumber int
	ETag       string
}

// Complete 完成上传：认领 operation → 完成/确认对象 → 在同一 DB 事务中创建 node 并写完成回执。
func (u *UploadSessionUseCase) Complete(ctx context.Context, cmd CompleteUploadSessionCommand) (domainnode.Node, error) {
	operationID, err := normalizeUploadClientOperationID(cmd.UploadID, cmd.ClientOperationID)
	if err != nil {
		return domainnode.Node{}, err
	}
	preflight, err := u.loadOwnedSession(ctx, cmd.Actor, cmd.UploadID)
	if err != nil {
		return domainnode.Node{}, err
	}
	if preflight.Status == domainsession.StatusCommitted {
		if preflight.ClientOperationID != operationID {
			return domainnode.Node{}, fmt.Errorf("%w: upload session belongs to another completion operation", ErrConflict)
		}
		return decodeUploadCompletionNode(preflight)
	}
	if preflight.Mode == domainsession.ModeMultipart {
		if _, err := normalizeCompletedParts(cmd.Parts); err != nil {
			return domainnode.Node{}, err
		}
	}
	session, err := u.claimCompletion(ctx, cmd.Actor, cmd.UploadID, operationID)
	if err != nil {
		return domainnode.Node{}, err
	}
	if session.Status == domainsession.StatusCommitted {
		return decodeUploadCompletionNode(session)
	}

	store, err := u.registry.Get(session.StorageProvider)
	if err != nil {
		u.releaseOperationClaim(session.ID, operationID)
		return domainnode.Node{}, fmt.Errorf("storage provider %q: %w", session.StorageProvider, err)
	}
	if err := completeUploadObject(ctx, store, session, cmd.Parts); err != nil {
		if errors.Is(err, ErrInvalidArgument) {
			u.releaseOperationClaim(session.ID, operationID)
		}
		return domainnode.Node{}, err
	}

	base := extractUploadBaseName(session.FileName)
	extWithDot := path.Ext(base)
	name := strings.TrimSuffix(base, extWithDot)
	if name == "" {
		name = base
	}
	ext := strings.TrimPrefix(extWithDot, ".")

	createNodeCommand := CreateNodeCommand{
		Actor:           cmd.Actor,
		Name:            name,
		Type:            domainnode.TypeFile,
		ParentID:        session.ParentID,
		LibraryID:       session.LibraryID,
		Ext:             ext,
		MIMEType:        session.ContentType,
		FileSize:        session.FileSize,
		StorageKey:      session.StorageKey,
		StorageProvider: session.StorageProvider,
		StorageBucket:   store.Bucket(),
		ConflictPolicy:  cmd.ConflictPolicy,
	}

	var (
		node      domainnode.Node
		committed bool
	)
	err = u.withinTx(ctx, func(txCtx context.Context) error {
		current, loadErr := u.repo.GetForUpdate(txCtx, session.ID)
		if loadErr != nil {
			return mapUploadSessionLoadError(loadErr)
		}
		if current.ActorID != cmd.Actor.ID {
			return fmt.Errorf("%w: upload session not found", ErrNotFound)
		}
		if current.ClientOperationID != operationID {
			return fmt.Errorf("%w: upload session belongs to another completion operation", ErrConflict)
		}
		if current.Status == domainsession.StatusCommitted {
			var decodeErr error
			node, decodeErr = decodeUploadCompletionNode(current)
			return decodeErr
		}

		created, createErr := u.nodes.createInExistingTransaction(txCtx, createNodeCommand)
		if createErr != nil {
			return createErr
		}
		resultJSON, marshalErr := json.Marshal(created)
		if marshalErr != nil {
			return fmt.Errorf("marshal upload completion result: %w", marshalErr)
		}
		completedAt := time.Now().UTC()
		ok, markErr := u.repo.MarkCommitted(
			txCtx,
			current.ID,
			operationID,
			created.ID,
			string(resultJSON),
			completedAt,
			completedAt.Add(uploadCompletionReceiptTTL),
		)
		if markErr != nil {
			return fmt.Errorf("persist upload completion receipt: %w", markErr)
		}
		if !ok {
			return fmt.Errorf("%w: upload completion state changed", ErrConflict)
		}
		node = created
		committed = true
		return nil
	})
	if err != nil {
		u.releaseOperationClaim(session.ID, operationID)
		return domainnode.Node{}, err
	}

	if committed {
		u.nodes.recordCreateSuccess(ctx, createNodeCommand, node)
		_ = u.writeAudit(ctx, cmd.Actor, "upload.completed", true, map[string]any{
			"library_id":   session.LibraryID,
			"parent_id":    session.ParentID,
			"node_id":      node.ID,
			"name":         node.Name,
			"storage_key":  session.StorageKey,
			"size":         session.FileSize,
			"mime_type":    session.ContentType,
			"mode":         session.Mode,
			"operation_id": operationID,
		})
		slog.InfoContext(ctx, "upload_session.complete",
			"upload_id", session.ID,
			"operation_id", operationID,
			"library_id", session.LibraryID,
			"node_id", node.ID,
			"size", session.FileSize,
		)
	}
	return node, nil
}

// ReconcileCompletion 查询 complete 的权威结果。未命中与跨 actor 查询都返回 unknown。
func (u *UploadSessionUseCase) ReconcileCompletion(
	ctx context.Context,
	act actor.Actor,
	clientOperationID string,
) (UploadCompletionStatusResult, error) {
	operationID := strings.TrimSpace(clientOperationID)
	if operationID == "" || len(operationID) > maxUploadClientOperationIDLength {
		return UploadCompletionStatusResult{}, fmt.Errorf("%w: valid clientOperationId is required", ErrInvalidArgument)
	}
	session, err := u.repo.GetByOperationIDAndActor(ctx, act.ID, operationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return UploadCompletionStatusResult{State: UploadCompletionStateUnknown}, nil
		}
		return UploadCompletionStatusResult{}, err
	}
	if !session.ExpiresAt.IsZero() && time.Now().UTC().After(session.ExpiresAt) {
		return UploadCompletionStatusResult{State: UploadCompletionStateUnknown}, nil
	}
	if session.Status != domainsession.StatusCommitted {
		return UploadCompletionStatusResult{State: UploadCompletionStateUncommitted}, nil
	}
	node, err := decodeUploadCompletionNode(session)
	if err != nil {
		return UploadCompletionStatusResult{}, err
	}
	return UploadCompletionStatusResult{
		State: UploadCompletionStateCommitted,
		Node:  &node,
	}, nil
}

func (u *UploadSessionUseCase) claimCompletion(
	ctx context.Context,
	act actor.Actor,
	uploadID string,
	operationID string,
) (domainsession.UploadSession, error) {
	if u.repo == nil {
		return domainsession.UploadSession{}, fmt.Errorf("%w: upload session repository not configured", ErrInvalidArgument)
	}
	var claimed domainsession.UploadSession
	err := u.withinTx(ctx, func(txCtx context.Context) error {
		session, loadErr := u.repo.GetForUpdate(txCtx, strings.TrimSpace(uploadID))
		if loadErr != nil {
			return mapUploadSessionLoadError(loadErr)
		}
		if session.ActorID != act.ID {
			return fmt.Errorf("%w: upload session not found", ErrNotFound)
		}
		if !session.ExpiresAt.IsZero() && time.Now().UTC().After(session.ExpiresAt) {
			return fmt.Errorf("%w: upload session lease expired", ErrExpired)
		}
		if session.Status == domainsession.StatusCommitted {
			if session.ClientOperationID != operationID {
				return fmt.Errorf("%w: upload session belongs to another completion operation", ErrConflict)
			}
			claimed = session
			return nil
		}
		if session.Status != domainsession.StatusPending {
			return fmt.Errorf("%w: invalid upload session state", ErrConflict)
		}
		if session.ClientOperationID != "" && session.ClientOperationID != operationID {
			return fmt.Errorf("%w: upload session belongs to another completion operation", ErrConflict)
		}
		if session.ClientOperationID == "" {
			now := time.Now().UTC()
			claimExpiresAt := session.ExpiresAt
			if minimum := now.Add(uploadCompletionClaimMinTTL); claimExpiresAt.Before(minimum) {
				claimExpiresAt = minimum
			}
			ok, setErr := u.repo.ClaimOperation(txCtx, session.ID, operationID, claimExpiresAt)
			if setErr != nil {
				if errors.Is(setErr, repository.ErrConflict) {
					return fmt.Errorf("%w: clientOperationId is already in use", ErrConflict)
				}
				return fmt.Errorf("claim upload completion operation: %w", setErr)
			}
			if !ok {
				return fmt.Errorf("%w: upload completion state changed", ErrConflict)
			}
			session.ClientOperationID = operationID
			session.ExpiresAt = claimExpiresAt
		}
		claimed = session
		return nil
	})
	return claimed, err
}

func completeUploadObject(
	ctx context.Context,
	store storage.ObjectStorage,
	session domainsession.UploadSession,
	parts []CompletedPart,
) error {
	var completeErr error
	if session.Mode == domainsession.ModeMultipart {
		storageParts, err := normalizeCompletedParts(parts)
		if err != nil {
			return err
		}
		completeErr = store.CompleteMultipartUpload(
			ctx,
			session.StorageKey,
			session.MinioUploadID,
			storageParts,
		)
	} else if session.Mode != domainsession.ModeSingle {
		return fmt.Errorf("%w: unsupported upload mode %q", ErrInvalidArgument, session.Mode)
	}

	// HEAD 是 single 的完成校验，也是 multipart 响应丢失后的幂等恢复判据。
	info, statErr := store.StatObject(ctx, session.StorageKey)
	if statErr != nil {
		if completeErr != nil {
			return fmt.Errorf("complete multipart: %w", completeErr)
		}
		return fmt.Errorf("%w: object not uploaded yet", ErrInvalidArgument)
	}
	if info.Size != session.FileSize {
		return fmt.Errorf(
			"%w: stored size %d does not match declared size %d",
			ErrInvalidArgument,
			info.Size,
			session.FileSize,
		)
	}
	if completeErr != nil {
		slog.InfoContext(ctx, "upload_session.complete.storage_already_committed",
			"upload_id", session.ID,
			"storage_key", session.StorageKey,
		)
	}
	return nil
}

func normalizeCompletedParts(parts []CompletedPart) ([]storage.MultipartUploadPart, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("%w: parts must not be empty for multipart upload", ErrInvalidArgument)
	}
	storageParts := make([]storage.MultipartUploadPart, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for i, part := range parts {
		if part.PartNumber < 1 || strings.TrimSpace(part.ETag) == "" {
			return nil, fmt.Errorf("%w: each completed part requires partNumber >= 1 and etag", ErrInvalidArgument)
		}
		if _, exists := seen[part.PartNumber]; exists {
			return nil, fmt.Errorf("%w: duplicate completed part %d", ErrInvalidArgument, part.PartNumber)
		}
		seen[part.PartNumber] = struct{}{}
		storageParts[i] = storage.MultipartUploadPart{
			PartNumber: part.PartNumber,
			ETag:       strings.TrimSpace(part.ETag),
		}
	}
	sort.Slice(storageParts, func(i, j int) bool {
		return storageParts[i].PartNumber < storageParts[j].PartNumber
	})
	return storageParts, nil
}

func decodeUploadCompletionNode(session domainsession.UploadSession) (domainnode.Node, error) {
	if session.Status != domainsession.StatusCommitted || strings.TrimSpace(session.CompletionResult) == "" {
		return domainnode.Node{}, fmt.Errorf("%w: upload completion receipt is incomplete", ErrConflict)
	}
	var node domainnode.Node
	if err := json.Unmarshal([]byte(session.CompletionResult), &node); err != nil {
		return domainnode.Node{}, fmt.Errorf("decode upload completion receipt: %w", err)
	}
	if node.ID == 0 || node.ID != session.CompletedNodeID {
		return domainnode.Node{}, fmt.Errorf("%w: upload completion receipt node mismatch", ErrConflict)
	}
	return node, nil
}

func normalizeUploadClientOperationID(uploadID string, clientOperationID string) (string, error) {
	trimmedUploadID := strings.TrimSpace(uploadID)
	if trimmedUploadID == "" {
		return "", fmt.Errorf("%w: uploadId is required", ErrInvalidArgument)
	}
	operationID := strings.TrimSpace(clientOperationID)
	if operationID == "" {
		// 兼容旧客户端：uploadId 本身也是单会话稳定值，仍能获得 complete 幂等语义。
		operationID = "upload:" + trimmedUploadID
	}
	if len(operationID) > maxUploadClientOperationIDLength {
		return "", fmt.Errorf("%w: clientOperationId is too long", ErrInvalidArgument)
	}
	return operationID, nil
}

func mapUploadSessionLoadError(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("%w: upload session not found", ErrNotFound)
	}
	return err
}

func (u *UploadSessionUseCase) withinTx(ctx context.Context, fn func(context.Context) error) error {
	if u.tx == nil {
		return fmt.Errorf("%w: upload completion requires transaction manager", ErrInvalidArgument)
	}
	return u.tx.WithinTx(ctx, fn)
}

func (u *UploadSessionUseCase) releaseOperationClaim(uploadID string, operationID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := u.repo.ClearClientOperationID(ctx, uploadID, operationID); err != nil {
		slog.WarnContext(ctx, "upload_session.operation.release_claim_failed",
			"upload_id", uploadID,
			"operation_id", operationID,
			"error", err,
		)
	}
}
