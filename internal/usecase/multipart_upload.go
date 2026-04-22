package usecase

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime"
	"path"
	"strings"
	"sync"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	"omniflow-go/internal/authz"
	"omniflow-go/internal/config"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/storage"

	"github.com/google/uuid"
)

type multipartSession struct {
	UploadID       string
	StorageKey     string
	LibraryID      uint64
	ParentID       uint64
	FileName       string
	FileSize       int64
	ContentType    string
	ChunkSize      int64
	TotalParts     int
	ConflictPolicy NodeNameConflictPolicy
	Actor          actor.Actor
	CreatedAt      time.Time
}

// InitiateMultipartUploadCommand 发起分片上传的参数。
type InitiateMultipartUploadCommand struct {
	Actor          actor.Actor
	LibraryID      uint64
	ParentID       uint64
	FileName       string
	FileSize       int64
	ContentType    string
	ConflictPolicy NodeNameConflictPolicy
}

// InitiateMultipartUploadResult 发起分片上传的返回值。
type InitiateMultipartUploadResult struct {
	UploadID   string `json:"uploadId"`
	StorageKey string `json:"storageKey"`
	ChunkSize  int64  `json:"chunkSize"`
	TotalParts int    `json:"totalParts"`
}

// UploadPartCommand 上传单个分片的参数。
type UploadPartCommand struct {
	Actor      actor.Actor
	UploadID   string
	PartNumber int
	Body       io.Reader
	Size       int64
}

// UploadPartResult 上传单个分片的返回值。
type UploadPartResult struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
}

// CompleteMultipartUploadCommand 完成分片上传的参数。
type CompleteMultipartUploadCommand struct {
	Actor    actor.Actor
	UploadID string
	Parts    []CompletedPart
}

// CompletedPart 已完成分片的信息。
type CompletedPart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
}

// MultipartPartInfo 已上传分片的查询结果。
type MultipartPartInfo struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

// MultipartUploadUseCase 处理分片上传业务逻辑。
type MultipartUploadUseCase struct {
	mu         sync.RWMutex
	sessions   map[string]*multipartSession
	nodes      *NodeUseCase
	storage    storage.ObjectStorage
	authorizer authz.Authorizer
	auditLog   audit.Sink
	cfg        *config.Config
	stopCh     chan struct{}
}

// NewMultipartUploadUseCase 创建分片上传 UseCase，启动后台过期会话清理。
func NewMultipartUploadUseCase(
	nodes *NodeUseCase,
	st storage.ObjectStorage,
	authorizer authz.Authorizer,
	auditLog audit.Sink,
	cfg *config.Config,
) (*MultipartUploadUseCase, func()) {
	uc := &MultipartUploadUseCase{
		sessions:   make(map[string]*multipartSession),
		nodes:      nodes,
		storage:    st,
		authorizer: authorizer,
		auditLog:   auditLog,
		cfg:        cfg,
		stopCh:     make(chan struct{}),
	}
	go uc.sweepExpiredSessions()
	cleanup := func() {
		close(uc.stopCh)
	}
	return uc, cleanup
}

func (u *MultipartUploadUseCase) sweepExpiredSessions() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-u.stopCh:
			return
		case <-ticker.C:
			u.cleanupExpired()
		}
	}
}

func (u *MultipartUploadUseCase) cleanupExpired() {
	u.mu.Lock()
	var expired []*multipartSession
	for id, s := range u.sessions {
		if time.Since(s.CreatedAt) > u.cfg.Upload.SessionTTL {
			expired = append(expired, s)
			delete(u.sessions, id)
		}
	}
	u.mu.Unlock()

	for _, s := range expired {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := u.storage.AbortMultipartUpload(ctx, s.StorageKey, s.UploadID); err != nil {
			slog.Warn("multipart_upload.sweep.abort_failed",
				"upload_id", s.UploadID,
				"error", err,
			)
		}
		cancel()
	}
}

// Initiate 创建分片上传会话。
func (u *MultipartUploadUseCase) Initiate(
	ctx context.Context,
	cmd InitiateMultipartUploadCommand,
) (InitiateMultipartUploadResult, error) {
	fileName := cmd.FileName
	if cmd.LibraryID == 0 || strings.TrimSpace(fileName) == "" {
		return InitiateMultipartUploadResult{}, fmt.Errorf(
			"%w: library id and file name are required", ErrInvalidArgument,
		)
	}
	if cmd.FileSize <= 0 {
		return InitiateMultipartUploadResult{}, fmt.Errorf(
			"%w: file size must be > 0", ErrInvalidArgument,
		)
	}
	if u.storage == nil {
		return InitiateMultipartUploadResult{}, fmt.Errorf(
			"%w: object storage not configured", ErrInvalidArgument,
		)
	}
	if err := u.authorize(ctx, cmd.Actor, cmd.LibraryID, authz.ActionUpload); err != nil {
		return InitiateMultipartUploadResult{}, err
	}

	base := extractUploadBaseName(fileName)
	extWithDot := path.Ext(base)

	contentType := strings.TrimSpace(cmd.ContentType)
	if contentType == "" || strings.EqualFold(contentType, defaultUploadContentType) {
		byExt := strings.TrimSpace(mime.TypeByExtension(extWithDot))
		if byExt != "" {
			contentType = byExt
		}
	}
	if contentType == "" {
		contentType = defaultUploadContentType
	}

	storageKey := fmt.Sprintf("libraries/%d/%s%s", cmd.LibraryID, uuid.NewString(), extWithDot)
	chunkSize := u.cfg.Upload.ChunkSizeBytes
	totalParts := int(math.Ceil(float64(cmd.FileSize) / float64(chunkSize)))

	uploadID, err := u.storage.InitiateMultipartUpload(ctx, storageKey, contentType)
	if err != nil {
		return InitiateMultipartUploadResult{}, err
	}

	session := &multipartSession{
		UploadID:       uploadID,
		StorageKey:     storageKey,
		LibraryID:      cmd.LibraryID,
		ParentID:       cmd.ParentID,
		FileName:       fileName,
		FileSize:       cmd.FileSize,
		ContentType:    contentType,
		ChunkSize:      chunkSize,
		TotalParts:     totalParts,
		ConflictPolicy: cmd.ConflictPolicy,
		Actor:          cmd.Actor,
		CreatedAt:      time.Now(),
	}

	u.mu.Lock()
	u.sessions[uploadID] = session
	u.mu.Unlock()

	slog.InfoContext(ctx, "multipart_upload.initiated",
		"upload_id", uploadID,
		"library_id", cmd.LibraryID,
		"file_name", cmd.FileName,
		"file_size", cmd.FileSize,
		"chunk_size", chunkSize,
		"total_parts", totalParts,
	)

	return InitiateMultipartUploadResult{
		UploadID:   uploadID,
		StorageKey: storageKey,
		ChunkSize:  chunkSize,
		TotalParts: totalParts,
	}, nil
}

// UploadPart 上传单个分片。
func (u *MultipartUploadUseCase) UploadPart(
	ctx context.Context,
	cmd UploadPartCommand,
) (UploadPartResult, error) {
	session, err := u.getSession(cmd.UploadID)
	if err != nil {
		return UploadPartResult{}, err
	}
	if cmd.PartNumber < 1 || cmd.PartNumber > session.TotalParts {
		return UploadPartResult{}, fmt.Errorf(
			"%w: part number must be between 1 and %d", ErrInvalidArgument, session.TotalParts,
		)
	}

	etag, err := u.storage.UploadPart(
		ctx, session.StorageKey, session.UploadID, cmd.PartNumber, cmd.Body, cmd.Size,
	)
	if err != nil {
		return UploadPartResult{}, err
	}

	return UploadPartResult{
		PartNumber: cmd.PartNumber,
		ETag:       etag,
	}, nil
}

// Complete 合并所有分片，创建文件节点。
func (u *MultipartUploadUseCase) Complete(
	ctx context.Context,
	cmd CompleteMultipartUploadCommand,
) (domainnode.Node, error) {
	session, err := u.getSession(cmd.UploadID)
	if err != nil {
		return domainnode.Node{}, err
	}

	storageParts := make([]storage.MultipartUploadPart, len(cmd.Parts))
	for i, p := range cmd.Parts {
		storageParts[i] = storage.MultipartUploadPart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		}
	}

	if err := u.storage.CompleteMultipartUpload(
		ctx, session.StorageKey, session.UploadID, storageParts,
	); err != nil {
		return domainnode.Node{}, err
	}

	base := extractUploadBaseName(session.FileName)
	extWithDot := path.Ext(base)
	name := strings.TrimSuffix(base, extWithDot)
	if name == "" {
		name = base
	}
	ext := strings.TrimPrefix(extWithDot, ".")

	node, err := u.nodes.Create(ctx, CreateNodeCommand{
		Actor:          session.Actor,
		Name:           name,
		Type:           domainnode.TypeFile,
		ParentID:       session.ParentID,
		LibraryID:      session.LibraryID,
		Ext:            ext,
		MIMEType:       session.ContentType,
		FileSize:       session.FileSize,
		StorageKey:     session.StorageKey,
		ConflictPolicy: session.ConflictPolicy,
	})
	if err != nil {
		_ = u.storage.Delete(ctx, session.StorageKey)
		return domainnode.Node{}, err
	}

	u.mu.Lock()
	delete(u.sessions, cmd.UploadID)
	u.mu.Unlock()

	_ = u.writeAudit(ctx, session.Actor, "multipart_upload.completed", true, map[string]any{
		"library_id":  session.LibraryID,
		"parent_id":   session.ParentID,
		"node_id":     node.ID,
		"name":        node.Name,
		"storage_key": session.StorageKey,
		"size":        session.FileSize,
		"mime_type":   session.ContentType,
		"parts":       len(cmd.Parts),
	})
	slog.InfoContext(ctx, "multipart_upload.completed",
		"upload_id", cmd.UploadID,
		"library_id", session.LibraryID,
		"node_id", node.ID,
		"size", session.FileSize,
	)
	return node, nil
}

// Abort 取消分片上传。
func (u *MultipartUploadUseCase) Abort(
	ctx context.Context,
	act actor.Actor,
	uploadID string,
) error {
	session, err := u.getSession(uploadID)
	if err != nil {
		return err
	}

	if err := u.storage.AbortMultipartUpload(ctx, session.StorageKey, session.UploadID); err != nil {
		return err
	}

	u.mu.Lock()
	delete(u.sessions, uploadID)
	u.mu.Unlock()

	slog.InfoContext(ctx, "multipart_upload.aborted",
		"upload_id", uploadID,
		"library_id", session.LibraryID,
	)
	return nil
}

// ListParts 列出已上传的分片。
func (u *MultipartUploadUseCase) ListParts(
	ctx context.Context,
	act actor.Actor,
	uploadID string,
) ([]MultipartPartInfo, error) {
	session, err := u.getSession(uploadID)
	if err != nil {
		return nil, err
	}

	parts, err := u.storage.ListParts(ctx, session.StorageKey, session.UploadID)
	if err != nil {
		return nil, err
	}

	result := make([]MultipartPartInfo, len(parts))
	for i, p := range parts {
		result[i] = MultipartPartInfo{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
			Size:       p.Size,
		}
	}
	return result, nil
}

func (u *MultipartUploadUseCase) getSession(uploadID string) (*multipartSession, error) {
	u.mu.RLock()
	session, ok := u.sessions[uploadID]
	u.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: upload session not found", ErrNotFound)
	}
	return session, nil
}

func (u *MultipartUploadUseCase) authorize(
	ctx context.Context,
	principal actor.Actor,
	libraryID uint64,
	action authz.Action,
) error {
	if u.authorizer == nil {
		return nil
	}
	return u.authorizer.Authorize(ctx, principal, authz.Resource{
		Kind: "library",
		ID:   fmt.Sprintf("%d", libraryID),
	}, action)
}

func (u *MultipartUploadUseCase) writeAudit(
	ctx context.Context,
	principal actor.Actor,
	action string,
	success bool,
	metadata map[string]any,
) error {
	if u.auditLog == nil {
		return nil
	}
	return u.auditLog.Write(ctx, audit.Event{
		Actor:      principal,
		Action:     action,
		Resource:   "directory",
		Success:    success,
		OccurredAt: time.Now().UTC(),
		Metadata:   metadata,
	})
}
