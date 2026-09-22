package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"omniflow-go/internal/actor"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/repository"
)

// RenameBatchCommand 将已批准的名称快照绑定到稳定操作身份。
type RenameBatchCommand struct {
	Actor       actor.Actor
	LibraryID   uint64
	OperationID string
	Items       []domainnode.RenameBatchItem
	DryRun      bool
}

func validRenameOperation(libraryID uint64, operationID string) error {
	id, err := uuid.Parse(operationID)
	if libraryID == 0 || libraryID > math.MaxInt64 || err != nil || id == uuid.Nil || id.String() != operationID {
		return fmt.Errorf("%w: libraryId and a canonical nonzero operationId UUID are required", ErrInvalidArgument)
	}
	return nil
}

func validateRenameBatch(cmd RenameBatchCommand) ([]domainnode.RenameBatchItem, string, error) {
	if err := validRenameOperation(cmd.LibraryID, cmd.OperationID); err != nil {
		return nil, "", err
	}
	if len(cmd.Items) == 0 || len(cmd.Items) > 50 {
		return nil, "", fmt.Errorf("%w: provide 1 to 50 files", ErrInvalidArgument)
	}
	items := append([]domainnode.RenameBatchItem(nil), cmd.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].NodeID < items[j].NodeID })
	for i := range items {
		item := &items[i]
		e := item.Expected
		if item.NodeID == 0 || item.NodeID > math.MaxInt64 || (i > 0 && items[i-1].NodeID == item.NodeID) ||
			e.ParentID == 0 || e.ParentID > math.MaxInt64 || e.Name == "" || e.UpdatedAt.IsZero() ||
			e.UpdatedAt.Year() < 1970 || e.UpdatedAt.Year() > 9999 || utf8.RuneCountInString(e.Name) > maxNodeNameLength ||
			utf8.RuneCountInString(e.Ext) > maxNodeExtLength {
			return nil, "", fmt.Errorf("%w: unique file IDs and complete expected snapshots are required", ErrInvalidArgument)
		}
		if !validRenameBase(item.Name) {
			return nil, "", fmt.Errorf("%w: name must be a valid file basename", ErrInvalidArgument)
		}
		// 统一时间表示；不能截断精度来接受过期的版本条件。
		item.Expected.UpdatedAt = item.Expected.UpdatedAt.UTC()
	}
	encoded, err := json.Marshal(struct {
		LibraryID uint64                       `json:"libraryId"`
		Items     []domainnode.RenameBatchItem `json:"items"`
	}{cmd.LibraryID, items})
	if err != nil {
		return nil, "", err
	}
	hash := sha256.Sum256(encoded)
	return items, hex.EncodeToString(hash[:]), nil
}

func validRenameBase(name string) bool {
	if name == "" || strings.TrimSpace(name) != name || name == "." || name == ".." ||
		utf8.RuneCountInString(name) > maxNodeNameLength || strings.ContainsAny(name, "/\\") {
		return false
	}
	return strings.IndexFunc(name, unicode.IsControl) == -1
}

func renameActorID(principal actor.Actor) (string, error) {
	if principal.ID == "" || principal.Kind == "" || principal.Kind == actor.KindAnonymous {
		return "", ErrUnauthorized
	}
	return string(principal.Kind) + ":" + principal.ID, nil
}

// RenameBatch 原子校验、修改及记录回执；取消或提交错误后必须查询回执核对实际结果。
func (u *NodeUseCase) RenameBatch(ctx context.Context, cmd RenameBatchCommand) (domainnode.RenameBatchResult, error) {
	items, hash, err := validateRenameBatch(cmd)
	if err != nil {
		return domainnode.RenameBatchResult{}, err
	}
	actorID, err := renameActorID(cmd.Actor)
	if err != nil {
		return domainnode.RenameBatchResult{}, err
	}
	if err := u.ensureNodesConfigured(); err != nil {
		return domainnode.RenameBatchResult{}, err
	}
	if u.tx == nil {
		return domainnode.RenameBatchResult{}, errors.New("conditional rename requires transaction manager")
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return domainnode.RenameBatchResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var result domainnode.RenameBatchResult
	callbackComplete := false
	err = u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		if err := u.nodes.LockMutationOperation(txCtx, actorID, cmd.OperationID); err != nil {
			return err
		}
		receipt, err := u.nodes.ReadMutationReceipt(txCtx, actorID, cmd.OperationID)
		if err == nil {
			if receipt.Kind != "rename_batch" || receipt.LibraryID != cmd.LibraryID || receipt.RequestHash != hash {
				return fmt.Errorf("%w: operationId is already bound to a different request", ErrConflict)
			}
			if cmd.DryRun {
				return fmt.Errorf("%w: operation already committed; query its status", ErrConflict)
			}
			if err := json.Unmarshal([]byte(receipt.Result), &result); err != nil {
				return fmt.Errorf("decode rename receipt: %w", err)
			}
			result.Replayed = true
			callbackComplete = true
			return nil
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		result, err = u.executeRenameBatch(txCtx, cmd, items)
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if err := u.nodes.SaveMutationReceipt(txCtx, domainnode.MutationReceipt{
			ActorID: actorID, OperationID: cmd.OperationID, Kind: "rename_batch", LibraryID: cmd.LibraryID,
			RequestHash: hash, Result: string(encoded),
		}); err != nil {
			return err
		}
		callbackComplete = true
		return nil
	})
	if err != nil {
		slog.WarnContext(ctx, "node.rename_batch.failed", "operation_id", cmd.OperationID, "library_id", cmd.LibraryID,
			"mode", resolveMutationMode(cmd.DryRun), "commit_unknown", callbackComplete && !cmd.DryRun, "error", err)
		_ = u.writeAudit(ctx, cmd.Actor, "node.rename_batch", false, map[string]any{
			"operation_id": cmd.OperationID, "library_id": cmd.LibraryID, "mode": resolveMutationMode(cmd.DryRun),
			"commit_unknown": callbackComplete && !cmd.DryRun,
		})
		if callbackComplete && !cmd.DryRun {
			return domainnode.RenameBatchResult{}, fmt.Errorf("rename commit outcome unknown; query operationId=%s: %w", cmd.OperationID, err)
		}
		return domainnode.RenameBatchResult{}, mapRenameBatchError(err)
	}
	if cmd.DryRun {
		result.State = "validated"
		for i := range result.Items {
			result.Items[i].Status = "validated"
			result.Items[i].Current.UpdatedAt = result.Items[i].Previous.UpdatedAt
		}
	}
	slog.InfoContext(ctx, "node.rename_batch.completed", "operation_id", cmd.OperationID, "library_id", cmd.LibraryID,
		"mode", resolveMutationMode(cmd.DryRun), "count", len(result.Items), "replayed", result.Replayed)
	_ = u.writeAudit(ctx, cmd.Actor, "node.rename_batch", true, map[string]any{
		"operation_id": cmd.OperationID, "library_id": cmd.LibraryID, "mode": resolveMutationMode(cmd.DryRun),
		"count": len(result.Items), "replayed": result.Replayed,
	})
	return result, nil
}

func (u *NodeUseCase) executeRenameBatch(ctx context.Context, cmd RenameBatchCommand, items []domainnode.RenameBatchItem) (domainnode.RenameBatchResult, error) {
	ids := make([]uint64, len(items))
	for i, item := range items {
		ids[i] = item.NodeID
	}
	nodes, err := u.nodes.LockRenameNodes(ctx, cmd.LibraryID, ids)
	if err != nil {
		return domainnode.RenameBatchResult{}, err
	}
	if len(nodes) != len(items) {
		return domainnode.RenameBatchResult{}, ErrNotFound
	}
	targets := map[string]bool{}
	parents := map[uint64]bool{}
	for i, item := range items {
		n := nodes[i]
		if n.ID != item.NodeID || n.Type != domainnode.TypeFile || n.ParentID == 0 || n.ArchiveMode != 0 {
			return domainnode.RenameBatchResult{}, fmt.Errorf("%w: only ordinary files are supported", ErrInvalidArgument)
		}
		e := item.Expected
		if n.Name != e.Name || n.Ext != e.Ext || n.ParentID != e.ParentID || !n.UpdatedAt.Equal(e.UpdatedAt) {
			return domainnode.RenameBatchResult{}, fmt.Errorf("%w: node %d changed after approval", ErrConflict, n.ID)
		}
		target := fmt.Sprintf("%d:%s", n.ParentID, nodeVisibleName(item.Name, n.Ext, n.Type))
		if targets[target] {
			return domainnode.RenameBatchResult{}, errNodeNameAlreadyExists
		}
		targets[target] = true
		parents[n.ParentID] = true
		taken, err := u.nodes.RenameNameTaken(ctx, cmd.LibraryID, n.ParentID, n.ID, item.Name, n.Ext)
		if err != nil {
			return domainnode.RenameBatchResult{}, err
		}
		if taken {
			return domainnode.RenameBatchResult{}, errNodeNameAlreadyExists
		}
	}
	result := domainnode.RenameBatchResult{OperationID: cmd.OperationID, LibraryID: cmd.LibraryID,
		Atomic: true, State: "committed", Items: make([]domainnode.RenameItemResult, 0, len(items)), AffectedParentIDs: []uint64{}}
	for i, item := range items {
		n := nodes[i]
		updated, err := u.nodes.WriteLockedRename(ctx, cmd.LibraryID, n.ID, item.Name)
		if err != nil {
			return domainnode.RenameBatchResult{}, err
		}
		result.Items = append(result.Items, domainnode.RenameItemResult{NodeID: n.ID, ParentID: n.ParentID, Status: "committed",
			Previous: domainnode.RenameState{Name: n.Name, Ext: n.Ext, UpdatedAt: n.UpdatedAt},
			Current:  domainnode.RenameState{Name: updated.Name, Ext: updated.Ext, UpdatedAt: updated.UpdatedAt}})
	}
	for parentID := range parents {
		result.AffectedParentIDs = append(result.AffectedParentIDs, parentID)
	}
	sort.Slice(result.AffectedParentIDs, func(i, j int) bool { return result.AffectedParentIDs[i] < result.AffectedParentIDs[j] })
	return result, nil
}

func mapRenameBatchError(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, repository.ErrConflict) {
		return errNodeNameAlreadyExists
	}
	return err
}

// RenameBatchStatus 仅返回当前 actor、本资料库的完成回执，不把未找到解释成未提交。
func (u *NodeUseCase) RenameBatchStatus(ctx context.Context, principal actor.Actor, libraryID uint64, operationID string) (domainnode.RenameBatchStatus, error) {
	if err := validRenameOperation(libraryID, operationID); err != nil {
		return domainnode.RenameBatchStatus{}, err
	}
	actorID, err := renameActorID(principal)
	if err != nil {
		return domainnode.RenameBatchStatus{}, err
	}
	if err := u.ensureNodesConfigured(); err != nil {
		return domainnode.RenameBatchStatus{}, err
	}
	if err := u.AuthorizeMutation(ctx, principal, libraryID); err != nil {
		return domainnode.RenameBatchStatus{}, err
	}
	status := domainnode.RenameBatchStatus{LibraryID: libraryID, OperationID: operationID, State: "not_found"}
	receipt, err := u.nodes.ReadMutationReceipt(ctx, actorID, operationID)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && (receipt.LibraryID != libraryID || receipt.Kind != "rename_batch")) {
		return status, nil
	}
	if err != nil {
		return domainnode.RenameBatchStatus{}, err
	}
	var result domainnode.RenameBatchResult
	if err := json.Unmarshal([]byte(receipt.Result), &result); err != nil {
		return domainnode.RenameBatchStatus{}, err
	}
	status.State, status.Result = "committed", &result
	return status, nil
}
