package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	"omniflow-go/internal/authz"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/repository"
)

type ListChildrenQuery struct {
	Actor     actor.Actor
	LibraryID uint64
	NodeID    uint64
}

type SearchNodesQuery struct {
	Actor        actor.Actor
	LibraryID    uint64
	Keyword      string
	TagIDs       []uint64
	TagMatchMode string
	Limit        int
}

type NodePath struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Depth int    `json:"depth"`
}

type CreateNodeCommand struct {
	Actor      actor.Actor
	Name       string
	Type       domainnode.Type
	ParentID   uint64
	LibraryID  uint64
	Ext        string
	MIMEType   string
	FileSize   int64
	StorageKey string
	DryRun     bool
}

type UpdateNodeCommand struct {
	Actor       actor.Actor
	BuiltInType *string
	ArchiveMode *int
	ViewMeta    *string
}

type RenameNodeCommand struct {
	Actor  actor.Actor
	Name   string
	Ext    *string
	DryRun bool
}

type MoveNodeCommand struct {
	Actor        actor.Actor
	LibraryID    uint64
	NodeID       uint64
	NewParentID  uint64
	BeforeNodeID uint64
	Name         string
	DryRun       bool
}

type SortComicChildrenCommand struct {
	Actor  actor.Actor
	NodeID uint64
}

type DeleteNodeTreeCommand struct {
	Actor     actor.Actor
	LibraryID uint64
	NodeID    uint64
	DryRun    bool
}

type RestoreNodeTreeCommand struct {
	Actor     actor.Actor
	LibraryID uint64
	NodeID    uint64
	DryRun    bool
}

type HardDeleteNodeTreeCommand struct {
	Actor     actor.Actor
	LibraryID uint64
	NodeID    uint64
	DryRun    bool
}

type NodeUseCase struct {
	nodes      *repository.NodeRepository
	tx         repository.Transactor
	authorizer authz.Authorizer
	auditLog   audit.Sink
}

func NewNodeUseCase(
	nodes *repository.NodeRepository,
	tx repository.Transactor,
	authorizer authz.Authorizer,
	auditLog audit.Sink,
) *NodeUseCase {
	return &NodeUseCase{
		nodes:      nodes,
		tx:         tx,
		authorizer: authorizer,
		auditLog:   auditLog,
	}
}

const (
	defaultStorageProvider = "MINIO"
	defaultStorageBucket   = "my-bucket"
)

func (u *NodeUseCase) Create(ctx context.Context, cmd CreateNodeCommand) (domainnode.Node, error) {
	name := cmd.Name
	if cmd.LibraryID == 0 || strings.TrimSpace(name) == "" {
		return domainnode.Node{}, fmt.Errorf("%w: library id and name are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return domainnode.Node{}, err
	}

	var created domainnode.Node
	err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		parentID, err := u.resolveCreateParentID(txCtx, cmd.LibraryID, cmd.ParentID)
		if err != nil {
			return err
		}

		result, err := u.nodes.CreateNode(txCtx, repository.CreateNodeInput{
			Name:            name,
			Type:            cmd.Type,
			ParentID:        parentID,
			LibraryID:       cmd.LibraryID,
			Ext:             strings.TrimSpace(cmd.Ext),
			MIMEType:        strings.TrimSpace(cmd.MIMEType),
			FileSize:        cmd.FileSize,
			StorageKey:      strings.TrimSpace(cmd.StorageKey),
			BuiltInType:     "DEF",
			ArchiveMode:     false,
			StorageProvider: defaultStorageProvider,
			StorageBucket:   defaultStorageBucket,
		})
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			if errors.Is(err, repository.ErrConflict) {
				return ErrConflict
			}
			if errors.Is(err, repository.ErrInvalidState) {
				return fmt.Errorf("%w: invalid node create request", ErrInvalidArgument)
			}
			return err
		}
		created = result
		return nil
	})
	if err != nil {
		return domainnode.Node{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "node.create", true, map[string]any{
		"node_id":    created.ID,
		"library_id": created.LibraryID,
		"parent_id":  created.ParentID,
		"type":       created.Type,
		"name":       created.Name,
		"mode":       resolveMutationMode(cmd.DryRun),
		"dry_run":    cmd.DryRun,
	})
	return created, nil
}

func (u *NodeUseCase) GetAllDescendants(ctx context.Context, principal actor.Actor, nodeID, libraryID uint64) ([]domainnode.Node, error) {
	if libraryID == 0 || nodeID == 0 {
		return nil, fmt.Errorf("%w: node id and library id are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeRead(ctx, principal, libraryID); err != nil {
		return nil, err
	}

	rows, err := u.nodes.ListAllDescendants(ctx, nodeID, libraryID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return rows, nil
}

func (u *NodeUseCase) GetDirectChildren(ctx context.Context, principal actor.Actor, nodeID, libraryID uint64) ([]domainnode.Node, error) {
	if libraryID == 0 || nodeID == 0 {
		return nil, fmt.Errorf("%w: node id and library id are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeRead(ctx, principal, libraryID); err != nil {
		return nil, err
	}

	rows, err := u.nodes.ListDirectChildren(ctx, nodeID, libraryID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return rows, nil
}

func (u *NodeUseCase) SearchNodes(ctx context.Context, query SearchNodesQuery) ([]domainnode.Node, error) {
	if query.LibraryID == 0 {
		return nil, fmt.Errorf("%w: library id is required", ErrInvalidArgument)
	}
	if err := u.AuthorizeRead(ctx, query.Actor, query.LibraryID); err != nil {
		return nil, err
	}

	tagIDs := normalizePositiveUint64List(query.TagIDs)
	tagMatchMode := normalizeTagMatchMode(query.TagMatchMode)
	limit := normalizeSearchLimit(query.Limit)

	rows, err := u.nodes.SearchNodes(ctx, repository.SearchNodesInput{
		LibraryID:    query.LibraryID,
		Keyword:      strings.TrimSpace(query.Keyword),
		TagIDs:       tagIDs,
		TagMatchMode: tagMatchMode,
		Limit:        limit,
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (u *NodeUseCase) GetAncestors(ctx context.Context, principal actor.Actor, nodeID, libraryID uint64) ([]NodePath, error) {
	if libraryID == 0 || nodeID == 0 {
		return nil, fmt.Errorf("%w: node id and library id are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeRead(ctx, principal, libraryID); err != nil {
		return nil, err
	}

	rows, err := u.nodes.ListAncestors(ctx, nodeID, libraryID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	result := make([]NodePath, 0, len(rows))
	for _, item := range rows {
		result = append(result, NodePath{
			ID:    item.ID,
			Name:  item.Name,
			Depth: item.Depth,
		})
	}
	return result, nil
}

func (u *NodeUseCase) GetFullPath(ctx context.Context, principal actor.Actor, nodeID, libraryID uint64) (string, error) {
	ancestors, err := u.GetAncestors(ctx, principal, nodeID, libraryID)
	if err != nil {
		return "", err
	}
	if len(ancestors) == 0 {
		return "", ErrNotFound
	}

	var b strings.Builder
	for _, item := range ancestors {
		b.WriteString("/")
		b.WriteString(item.Name)
	}
	return b.String(), nil
}

func (u *NodeUseCase) GetLibraryRootNodeID(ctx context.Context, principal actor.Actor, libraryID uint64) (uint64, error) {
	if libraryID == 0 {
		return 0, fmt.Errorf("%w: library id is required", ErrInvalidArgument)
	}
	if err := u.AuthorizeRead(ctx, principal, libraryID); err != nil {
		return 0, err
	}

	rootNodeID, err := u.nodes.EnsureLibraryRootNodeID(ctx, libraryID)
	if err != nil {
		return 0, err
	}
	return rootNodeID, nil
}

func (u *NodeUseCase) GetNodeDetail(ctx context.Context, nodeID uint64) (domainnode.Node, error) {
	if nodeID == 0 {
		return domainnode.Node{}, fmt.Errorf("%w: node id is required", ErrInvalidArgument)
	}

	row, err := u.nodes.FindViewByNodeID(ctx, nodeID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domainnode.Node{}, ErrNotFound
		}
		return domainnode.Node{}, err
	}
	return row, nil
}

func (u *NodeUseCase) Update(ctx context.Context, nodeID uint64, cmd UpdateNodeCommand) error {
	if nodeID == 0 {
		return fmt.Errorf("%w: node id is required", ErrInvalidArgument)
	}

	node, err := u.GetNodeDetail(ctx, nodeID)
	if err != nil {
		return err
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, node.LibraryID); err != nil {
		return err
	}

	updates := map[string]any{}
	if cmd.BuiltInType != nil {
		builtInType := strings.TrimSpace(*cmd.BuiltInType)
		if builtInType == "" {
			builtInType = strings.TrimSpace(node.BuiltInType)
			if builtInType == "" {
				builtInType = "DEF"
			}
		} else {
			builtInType = strings.ToUpper(builtInType)
		}
		updates["built_in_type"] = builtInType
	}
	if cmd.ArchiveMode != nil {
		if *cmd.ArchiveMode != 0 && *cmd.ArchiveMode != 1 {
			return fmt.Errorf("%w: archive mode only supports 0 or 1", ErrInvalidArgument)
		}
		updates["archive_mode"] = (*cmd.ArchiveMode == 1)
	}
	if cmd.ViewMeta != nil {
		viewMeta := strings.TrimSpace(*cmd.ViewMeta)
		if viewMeta == "" {
			viewMeta = "{}"
		}
		if !json.Valid([]byte(viewMeta)) {
			return fmt.Errorf("%w: viewMeta must be valid json", ErrInvalidArgument)
		}
		updates["view_meta"] = viewMeta
	}
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now().UTC()

	ok, err := u.nodes.UpdateNodeFields(ctx, nodeID, node.LibraryID, updates)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidState) {
			return fmt.Errorf("%w: invalid node update request", ErrInvalidArgument)
		}
		return err
	}
	if !ok {
		return ErrNotFound
	}

	_ = u.writeAudit(ctx, cmd.Actor, "node.update", true, map[string]any{
		"node_id":    nodeID,
		"library_id": node.LibraryID,
	})
	return nil
}

func (u *NodeUseCase) Rename(ctx context.Context, nodeID uint64, cmd RenameNodeCommand) error {
	newName := cmd.Name
	if nodeID == 0 || strings.TrimSpace(newName) == "" {
		return fmt.Errorf("%w: node id and name are required", ErrInvalidArgument)
	}

	node, err := u.GetNodeDetail(ctx, nodeID)
	if err != nil {
		return err
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, node.LibraryID); err != nil {
		return err
	}

	var ext *string
	if node.Type == domainnode.TypeFile && cmd.Ext != nil {
		trimmed := strings.TrimSpace(*cmd.Ext)
		ext = &trimmed
	}

	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		if err := u.nodes.RenameNode(txCtx, nodeID, node.LibraryID, newName, ext, time.Now().UTC()); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			if errors.Is(err, repository.ErrConflict) {
				return ErrConflict
			}
			if errors.Is(err, repository.ErrInvalidState) {
				return fmt.Errorf("%w: invalid node rename request", ErrInvalidArgument)
			}
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "node.rename", true, map[string]any{
		"node_id":    nodeID,
		"library_id": node.LibraryID,
		"name":       newName,
		"mode":       resolveMutationMode(cmd.DryRun),
		"dry_run":    cmd.DryRun,
	})
	return nil
}

func (u *NodeUseCase) Move(ctx context.Context, cmd MoveNodeCommand) error {
	if cmd.LibraryID == 0 || cmd.NodeID == 0 {
		return fmt.Errorf("%w: library id and node id are required", ErrInvalidArgument)
	}
	if cmd.NewParentID == 0 {
		return fmt.Errorf("%w: new parent id is required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return err
	}

	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		if cmd.BeforeNodeID > 0 && cmd.BeforeNodeID == cmd.NodeID {
			// Java 语义：beforeNode 指向自己时直接视为 no-op。
			return nil
		}

		if err := u.nodes.MoveNode(txCtx, repository.MoveNodeInput{
			LibraryID:    cmd.LibraryID,
			NodeID:       cmd.NodeID,
			NewParentID:  cmd.NewParentID,
			BeforeNodeID: cmd.BeforeNodeID,
			Name:         cmd.Name,
			UpdatedAt:    time.Now().UTC(),
		}); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			if errors.Is(err, repository.ErrConflict) {
				return ErrConflict
			}
			if errors.Is(err, repository.ErrInvalidState) {
				return fmt.Errorf("%w: invalid node move request", ErrInvalidArgument)
			}
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	_ = u.RecordMoveIntent(ctx, cmd)
	_ = u.writeAudit(ctx, cmd.Actor, "node.move", true, map[string]any{
		"node_id":        cmd.NodeID,
		"library_id":     cmd.LibraryID,
		"new_parent_id":  cmd.NewParentID,
		"before_node_id": cmd.BeforeNodeID,
		"mode":           resolveMutationMode(cmd.DryRun),
		"dry_run":        cmd.DryRun,
	})
	return nil
}

func (u *NodeUseCase) SortComicChildrenByName(ctx context.Context, cmd SortComicChildrenCommand) error {
	if cmd.NodeID == 0 {
		return fmt.Errorf("%w: node id is required", ErrInvalidArgument)
	}

	node, err := u.GetNodeDetail(ctx, cmd.NodeID)
	if err != nil {
		return err
	}
	if node.Type != domainnode.TypeDirectory {
		return fmt.Errorf("%w: target node must be a directory", ErrInvalidArgument)
	}

	builtInType := strings.ToUpper(strings.TrimSpace(node.BuiltInType))
	if builtInType == "" {
		builtInType = "DEF"
	}
	if builtInType != "COMIC" {
		return fmt.Errorf("%w: only COMIC directories support name sorting", ErrInvalidArgument)
	}

	if err := u.AuthorizeMutation(ctx, cmd.Actor, node.LibraryID); err != nil {
		return err
	}

	if err := u.withinTx(ctx, func(txCtx context.Context) error {
		if err := u.nodes.SortDirectChildrenByName(txCtx, cmd.NodeID, node.LibraryID, time.Now().UTC()); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			if errors.Is(err, repository.ErrInvalidState) {
				return fmt.Errorf("%w: invalid comic sort request", ErrInvalidArgument)
			}
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "node.sort_comic_children", true, map[string]any{
		"node_id":    cmd.NodeID,
		"library_id": node.LibraryID,
	})
	return nil
}

func (u *NodeUseCase) DeleteNodeAndChildren(ctx context.Context, cmd DeleteNodeTreeCommand) (bool, error) {
	if cmd.LibraryID == 0 || cmd.NodeID == 0 {
		return false, fmt.Errorf("%w: library id and node id are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return false, err
	}

	var deleteResult repository.DeleteNodeTreeResult
	err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		result, err := u.nodes.DeleteTree(txCtx, cmd.NodeID, cmd.LibraryID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}
		deleteResult = result
		return nil
	})
	if err != nil {
		return false, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "node.delete_tree", true, map[string]any{
		"node_id":       cmd.NodeID,
		"library_id":    cmd.LibraryID,
		"deleted_nodes": deleteResult.DeletedNodeCount,
		"file_nodes":    deleteResult.FileNodeCount,
		"mode":          resolveMutationMode(cmd.DryRun),
		"dry_run":       cmd.DryRun,
	})
	return true, nil
}

func (u *NodeUseCase) GetRecycleBinItems(ctx context.Context, libraryID uint64) ([]domainnode.RecycleItem, error) {
	if libraryID == 0 {
		return nil, fmt.Errorf("%w: library id is required", ErrInvalidArgument)
	}

	rows, err := u.nodes.ListDeletedNodes(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []domainnode.RecycleItem{}, nil
	}

	deletedSet := make(map[uint64]struct{}, len(rows))
	childrenByParent := make(map[uint64][]uint64, len(rows))
	for _, item := range rows {
		deletedSet[item.ID] = struct{}{}
	}
	for _, item := range rows {
		if item.ParentID == 0 {
			continue
		}
		if _, ok := deletedSet[item.ParentID]; !ok {
			continue
		}
		childrenByParent[item.ParentID] = append(childrenByParent[item.ParentID], item.ID)
	}

	topLevel := make([]domainnode.RecycleItem, 0, len(rows))
	for _, item := range rows {
		if item.ParentID == 0 {
			topLevel = append(topLevel, item)
			continue
		}
		if _, ok := deletedSet[item.ParentID]; !ok {
			topLevel = append(topLevel, item)
		}
	}

	for i := range topLevel {
		subtreeCount := countDeletedSubtree(childrenByParent, topLevel[i].ID)
		if subtreeCount > 1 {
			topLevel[i].DeletedDescendantCount = subtreeCount - 1
		}
	}

	sort.Slice(topLevel, func(i, j int) bool {
		if !topLevel[i].DeletedAt.Equal(topLevel[j].DeletedAt) {
			return topLevel[i].DeletedAt.After(topLevel[j].DeletedAt)
		}
		return topLevel[i].ID > topLevel[j].ID
	})
	return topLevel, nil
}

func (u *NodeUseCase) RestoreNodeAndChildren(ctx context.Context, cmd RestoreNodeTreeCommand) (bool, error) {
	if cmd.LibraryID == 0 || cmd.NodeID == 0 {
		return false, fmt.Errorf("%w: library id and node id are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return false, err
	}

	var ok bool
	err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		result, err := u.nodes.RestoreTree(txCtx, cmd.NodeID, cmd.LibraryID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			if errors.Is(err, repository.ErrConflict) {
				return ErrConflict
			}
			if errors.Is(err, repository.ErrInvalidState) {
				return fmt.Errorf("%w: invalid node restore request", ErrInvalidArgument)
			}
			return err
		}
		ok = result
		return nil
	})
	if err != nil {
		return false, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "node.restore_tree", true, map[string]any{
		"node_id":    cmd.NodeID,
		"library_id": cmd.LibraryID,
		"mode":       resolveMutationMode(cmd.DryRun),
		"dry_run":    cmd.DryRun,
	})
	return ok, nil
}

func (u *NodeUseCase) HardDeleteNodeAndChildren(ctx context.Context, cmd HardDeleteNodeTreeCommand) (bool, error) {
	if cmd.LibraryID == 0 || cmd.NodeID == 0 {
		return false, fmt.Errorf("%w: library id and node id are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return false, err
	}

	var ok bool
	err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		result, err := u.nodes.HardDeleteTree(txCtx, cmd.NodeID, cmd.LibraryID)
		if err != nil {
			if errors.Is(err, repository.ErrInvalidState) {
				return fmt.Errorf("%w: invalid node hard delete request", ErrInvalidArgument)
			}
			return err
		}
		ok = result
		return nil
	})
	if err != nil {
		return false, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "node.hard_delete_tree", true, map[string]any{
		"node_id":    cmd.NodeID,
		"library_id": cmd.LibraryID,
		"mode":       resolveMutationMode(cmd.DryRun),
		"dry_run":    cmd.DryRun,
	})
	return ok, nil
}

func (u *NodeUseCase) findNodeView(ctx context.Context, nodeID, libraryID uint64) (domainnode.Node, error) {
	row, err := u.nodes.FindViewByID(ctx, nodeID, libraryID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domainnode.Node{}, ErrNotFound
		}
		return domainnode.Node{}, err
	}
	return row, nil
}

func (u *NodeUseCase) withinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if u.tx == nil {
		return fn(ctx)
	}
	return u.tx.WithinTx(ctx, fn)
}

func (u *NodeUseCase) withinMutationTx(ctx context.Context, dryRun bool, fn func(ctx context.Context) error) error {
	if !dryRun {
		return u.withinTx(ctx, fn)
	}
	if u.tx == nil {
		return fmt.Errorf("%w: dry-run requires transaction manager", ErrInvalidArgument)
	}

	err := u.tx.WithinTx(ctx, func(txCtx context.Context) error {
		if err := fn(txCtx); err != nil {
			return err
		}
		return errUsecaseDryRunRollback
	})
	if err != nil && !errors.Is(err, errUsecaseDryRunRollback) {
		return err
	}
	return nil
}

func (u *NodeUseCase) AuthorizeMutation(ctx context.Context, principal actor.Actor, libraryID uint64) error {
	if u.authorizer == nil {
		return nil
	}

	return u.authorizer.Authorize(ctx, principal, authz.Resource{
		Kind: "library",
		ID:   fmt.Sprintf("%d", libraryID),
	}, authz.ActionWrite)
}

func (u *NodeUseCase) AuthorizeRead(ctx context.Context, principal actor.Actor, libraryID uint64) error {
	if u.authorizer == nil {
		return nil
	}

	return u.authorizer.Authorize(ctx, principal, authz.Resource{
		Kind: "library",
		ID:   fmt.Sprintf("%d", libraryID),
	}, authz.ActionRead)
}

func (u *NodeUseCase) RecordMoveIntent(ctx context.Context, cmd MoveNodeCommand) error {
	if u.auditLog == nil {
		return nil
	}

	return u.auditLog.Write(ctx, audit.Event{
		Actor:      cmd.Actor,
		Action:     "node.move.intent",
		Resource:   "node",
		Success:    true,
		OccurredAt: time.Now().UTC(),
		Metadata: map[string]any{
			"library_id":     cmd.LibraryID,
			"node_id":        cmd.NodeID,
			"new_parent_id":  cmd.NewParentID,
			"before_node_id": cmd.BeforeNodeID,
			"name":           cmd.Name,
			"mode":           resolveMutationMode(cmd.DryRun),
			"dry_run":        cmd.DryRun,
		},
	})
}

func (u *NodeUseCase) writeAudit(ctx context.Context, principal actor.Actor, action string, success bool, metadata map[string]any) error {
	if u.auditLog == nil {
		return nil
	}
	return u.auditLog.Write(ctx, audit.Event{
		Actor:      principal,
		Action:     action,
		Resource:   "node",
		Success:    success,
		OccurredAt: time.Now().UTC(),
		Metadata:   metadata,
	})
}

func countDeletedSubtree(childrenByParent map[uint64][]uint64, rootID uint64) int {
	if rootID == 0 {
		return 0
	}

	count := 0
	stack := []uint64{rootID}
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		count++

		children := childrenByParent[current]
		if len(children) > 0 {
			stack = append(stack, children...)
		}
	}
	return count
}

func normalizeTagMatchMode(rawMode string) string {
	mode := strings.ToUpper(strings.TrimSpace(rawMode))
	if mode == "ALL" {
		return "ALL"
	}
	return "ANY"
}

func (u *NodeUseCase) resolveCreateParentID(ctx context.Context, libraryID, parentID uint64) (uint64, error) {
	if parentID == 0 {
		return u.nodes.EnsureLibraryRootNodeID(ctx, libraryID)
	}

	parent, err := u.nodes.FindViewByID(ctx, parentID, libraryID)
	if err == nil {
		if parent.Type != domainnode.TypeDirectory {
			return 0, fmt.Errorf("%w: target parent must be directory", ErrInvalidArgument)
		}
		return parentID, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return 0, err
	}

	rootID, rootErr := u.nodes.EnsureLibraryRootNodeID(ctx, libraryID)
	if rootErr != nil {
		return 0, rootErr
	}
	root, rootLoadErr := u.nodes.FindViewByID(ctx, rootID, libraryID)
	if rootLoadErr != nil {
		if errors.Is(rootLoadErr, repository.ErrNotFound) {
			return 0, ErrNotFound
		}
		return 0, rootLoadErr
	}
	if root.Type != domainnode.TypeDirectory {
		return 0, fmt.Errorf("%w: target parent must be directory", ErrInvalidArgument)
	}
	return rootID, nil
}

func normalizeSearchLimit(rawLimit int) int {
	if rawLimit <= 0 {
		return 200
	}
	if rawLimit > 500 {
		return 500
	}
	return rawLimit
}

func normalizePositiveUint64List(values []uint64) []uint64 {
	if len(values) == 0 {
		return []uint64{}
	}

	unique := make(map[uint64]struct{}, len(values))
	result := make([]uint64, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := unique[value]; exists {
			continue
		}
		unique[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
