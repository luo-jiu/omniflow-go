package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"omniflow-go/internal/actor"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/repository"
)

// NodeTagDeltaCommand 明确增删直接标签，省略的标签和 metadata 保持不变。
type NodeTagDeltaCommand struct {
	Actor                   actor.Actor
	NodeID, LibraryID       uint64
	AddTagIDs, RemoveTagIDs []uint64
	DryRun                  bool
}

func validateNodeTagDelta(cmd NodeTagDeltaCommand) error {
	if cmd.NodeID == 0 || cmd.LibraryID == 0 || cmd.NodeID > math.MaxInt64 || cmd.LibraryID > math.MaxInt64 {
		return fmt.Errorf("%w: valid nodeId and libraryId are required", ErrInvalidArgument)
	}
	if len(cmd.AddTagIDs)+len(cmd.RemoveTagIDs) == 0 || len(cmd.AddTagIDs) > 100 || len(cmd.RemoveTagIDs) > 100 {
		return fmt.Errorf("%w: provide 1 to 100 ids per delta list", ErrInvalidArgument)
	}
	seen := map[uint64]bool{}
	for _, ids := range [][]uint64{cmd.AddTagIDs, cmd.RemoveTagIDs} {
		for _, id := range ids {
			if id == 0 || id > math.MaxInt64 || seen[id] {
				return fmt.Errorf("%w: tag ids must be positive, unique and disjoint", ErrInvalidArgument)
			}
			seen[id] = true
		}
	}
	return nil
}

func mapNodeTagError(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, repository.ErrInvalidState) {
		return fmt.Errorf("%w: tags unavailable, not owned or not bindable to this node", ErrInvalidArgument)
	}
	return err
}

// UpdateNodeTags 在同一事务校验、锁定、增删关系及同步兼容字段，dry-run 执行后回滚。
func (u *NodeUseCase) UpdateNodeTags(ctx context.Context, cmd NodeTagDeltaCommand) (domainnode.TagState, error) {
	if err := validateNodeTagDelta(cmd); err != nil {
		return domainnode.TagState{}, err
	}
	if err := u.ensureNodesConfigured(); err != nil {
		return domainnode.TagState{}, err
	}
	if u.tx == nil {
		return domainnode.TagState{}, fmt.Errorf("node tag mutation requires transaction manager")
	}
	ownerID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return domainnode.TagState{}, err
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return domainnode.TagState{}, err
	}
	var result domainnode.TagState
	err = u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		var mutationErr error
		result, mutationErr = u.nodes.ApplyNodeTagDelta(txCtx, cmd.NodeID, cmd.LibraryID, ownerID, cmd.AddTagIDs, cmd.RemoveTagIDs)
		return mutationErr
	})
	if err != nil {
		slog.WarnContext(ctx, "node.tags.update_failed", "node_id", cmd.NodeID, "library_id", cmd.LibraryID, "mode", resolveMutationMode(cmd.DryRun), "error", err)
		return domainnode.TagState{}, mapNodeTagError(err)
	}
	result.DryRun = cmd.DryRun
	slog.InfoContext(ctx, "node.tags.updated", "node_id", cmd.NodeID, "library_id", cmd.LibraryID, "mode", resolveMutationMode(cmd.DryRun))
	_ = u.writeAudit(ctx, cmd.Actor, "node.tags.update", true, map[string]any{"node_id": cmd.NodeID, "library_id": cmd.LibraryID,
		"add_count": len(cmd.AddTagIDs), "remove_count": len(cmd.RemoveTagIDs), "mode": resolveMutationMode(cmd.DryRun), "dry_run": cmd.DryRun})
	return result, nil
}

// ReadNodeTags 使用正式关系表读取当前资料库节点标签。
func (u *NodeUseCase) ReadNodeTags(ctx context.Context, principal actor.Actor, nodeID, libraryID uint64) (domainnode.TagState, error) {
	if nodeID == 0 || libraryID == 0 || nodeID > math.MaxInt64 || libraryID > math.MaxInt64 {
		return domainnode.TagState{}, ErrInvalidArgument
	}
	if err := u.ensureNodesConfigured(); err != nil {
		return domainnode.TagState{}, err
	}
	if u.tx == nil {
		return domainnode.TagState{}, fmt.Errorf("node tag read requires transaction manager")
	}
	if err := u.AuthorizeRead(ctx, principal, libraryID); err != nil {
		return domainnode.TagState{}, err
	}
	var result domainnode.TagState
	err := u.withinTx(ctx, func(txCtx context.Context) error {
		var readErr error
		result, readErr = u.nodes.ReadNodeTagState(txCtx, nodeID, libraryID)
		return readErr
	})
	return result, mapNodeTagError(err)
}
