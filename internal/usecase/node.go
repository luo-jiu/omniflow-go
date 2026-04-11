package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	"omniflow-go/internal/authz"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/repository"

	"github.com/samber/lo"
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

type MoveNodeBatchItemCommand struct {
	NodeID uint64
	Name   string
}

type MoveNodeBatchCommand struct {
	Actor        actor.Actor
	LibraryID    uint64
	NewParentID  uint64
	BeforeNodeID uint64
	Items        []MoveNodeBatchItemCommand
	DryRun       bool
}

type MoveNodeBatchResult struct {
	MovedCount        int      `json:"movedCount"`
	AffectedParentIDs []uint64 `json:"affectedParentIds"`
	MovedNodeIDs      []uint64 `json:"movedNodeIds"`
}

type SortComicChildrenCommand struct {
	Actor  actor.Actor
	NodeID uint64
}

type BatchSetArchiveChildrenBuiltInTypeCommand struct {
	Actor  actor.Actor
	NodeID uint64
}

type BatchSetArchiveChildrenBuiltInTypeResult struct {
	NodeID        uint64 `json:"nodeId"`
	LibraryID     uint64 `json:"libraryId"`
	BuiltInType   string `json:"builtInType"`
	TotalChildren int    `json:"totalChildren"`
	DirChildren   int    `json:"dirChildren"`
	UpdatedCount  int    `json:"updatedCount"`
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
	maxNodeExtLength       = 32
)

var errNodeRepositoryNotConfigured = errors.New("node repository is not configured")

func (u *NodeUseCase) Create(ctx context.Context, cmd CreateNodeCommand) (domainnode.Node, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return domainnode.Node{}, err
	}

	name := cmd.Name
	if cmd.LibraryID == 0 || strings.TrimSpace(name) == "" {
		return domainnode.Node{}, fmt.Errorf("%w: library id and name are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return domainnode.Node{}, err
	}

	normalizedExt := normalizeNodeExt(cmd.Ext)
	if normalizedExt != "" && utf8.RuneCountInString(normalizedExt) > maxNodeExtLength {
		return domainnode.Node{}, fmt.Errorf("%w: ext is too long", ErrInvalidArgument)
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
			Ext:             normalizedExt,
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
	slog.InfoContext(ctx, "node.created",
		"node_id", created.ID,
		"library_id", created.LibraryID,
		"parent_id", created.ParentID,
		"type", created.Type,
		"dry_run", cmd.DryRun,
	)
	return created, nil
}

func (u *NodeUseCase) GetAllDescendants(ctx context.Context, principal actor.Actor, nodeID, libraryID uint64) ([]domainnode.Node, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return nil, err
	}

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
	slog.DebugContext(ctx, "node.descendants.listed",
		"node_id", nodeID,
		"library_id", libraryID,
		"result_count", len(rows),
	)
	return rows, nil
}

func (u *NodeUseCase) GetDirectChildren(ctx context.Context, principal actor.Actor, nodeID, libraryID uint64) ([]domainnode.Node, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return nil, err
	}

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
	slog.DebugContext(ctx, "node.children.listed",
		"node_id", nodeID,
		"library_id", libraryID,
		"result_count", len(rows),
	)
	return rows, nil
}

func (u *NodeUseCase) SearchNodes(ctx context.Context, query SearchNodesQuery) ([]domainnode.Node, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return nil, err
	}

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
	slog.DebugContext(ctx, "node.search.completed",
		"library_id", query.LibraryID,
		"keyword", strings.TrimSpace(query.Keyword),
		"tag_count", len(tagIDs),
		"tag_match_mode", tagMatchMode,
		"limit", limit,
		"result_count", len(rows),
	)
	return rows, nil
}

func (u *NodeUseCase) GetAncestors(ctx context.Context, principal actor.Actor, nodeID, libraryID uint64) ([]NodePath, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return nil, err
	}

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

	result := lo.Map(rows, func(item repository.NodePathItem, _ int) NodePath {
		return NodePath{
			ID:    item.ID,
			Name:  item.Name,
			Depth: item.Depth,
		}
	})
	slog.DebugContext(ctx, "node.ancestors.listed",
		"node_id", nodeID,
		"library_id", libraryID,
		"result_count", len(result),
	)
	return result, nil
}

func (u *NodeUseCase) GetFullPath(ctx context.Context, principal actor.Actor, nodeID, libraryID uint64) (string, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return "", err
	}

	ancestors, err := u.GetAncestors(ctx, principal, nodeID, libraryID)
	if err != nil {
		return "", err
	}
	if len(ancestors) == 0 {
		return "", ErrNotFound
	}

	segments := lo.Map(ancestors, func(item NodePath, _ int) string {
		return item.Name
	})
	path := "/" + strings.Join(segments, "/")
	slog.DebugContext(ctx, "node.path.resolved",
		"node_id", nodeID,
		"library_id", libraryID,
		"path_depth", len(segments),
	)
	return path, nil
}

func (u *NodeUseCase) GetLibraryRootNodeID(ctx context.Context, principal actor.Actor, libraryID uint64) (uint64, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return 0, err
	}

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
	slog.DebugContext(ctx, "node.root.resolved",
		"library_id", libraryID,
		"root_node_id", rootNodeID,
	)
	return rootNodeID, nil
}

func (u *NodeUseCase) GetNodeDetail(ctx context.Context, principal actor.Actor, nodeID uint64) (domainnode.Node, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return domainnode.Node{}, err
	}

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
	if err := u.AuthorizeRead(ctx, principal, row.LibraryID); err != nil {
		return domainnode.Node{}, err
	}
	slog.DebugContext(ctx, "node.detail.fetched",
		"node_id", row.ID,
		"library_id", row.LibraryID,
		"type", row.Type,
	)
	return row, nil
}

func (u *NodeUseCase) Update(ctx context.Context, nodeID uint64, cmd UpdateNodeCommand) error {
	if err := u.ensureNodesConfigured(); err != nil {
		return err
	}

	if nodeID == 0 {
		return fmt.Errorf("%w: node id is required", ErrInvalidArgument)
	}

	node, err := u.GetNodeDetail(ctx, cmd.Actor, nodeID)
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

	nextViewMeta := node.ViewMeta
	if updatedViewMeta, ok := updates["view_meta"].(string); ok {
		nextViewMeta = updatedViewMeta
	}

	normalizedNextBuiltInType := strings.TrimSpace(node.BuiltInType)
	if cmd.BuiltInType != nil {
		normalizedNextBuiltInType = strings.ToUpper(strings.TrimSpace(*cmd.BuiltInType))
		if normalizedNextBuiltInType == "" {
			normalizedNextBuiltInType = strings.TrimSpace(node.BuiltInType)
			if normalizedNextBuiltInType == "" {
				normalizedNextBuiltInType = "DEF"
			}
		}
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

	if node.Type == domainnode.TypeDirectory && cmd.BuiltInType != nil {
		if normalizedBuiltIn := normalizeArchiveCardBuiltInType(normalizedNextBuiltInType); normalizedBuiltIn != "" {
			_ = u.warmupArchiveCoverMetaForNodes(ctx, node.LibraryID, normalizedBuiltIn, []ArchiveCardItem{
				{
					ID:       node.ID,
					ViewMeta: nextViewMeta,
				},
			})
		}
	}

	_ = u.writeAudit(ctx, cmd.Actor, "node.update", true, map[string]any{
		"node_id":    nodeID,
		"library_id": node.LibraryID,
	})
	slog.InfoContext(ctx, "node.updated",
		"node_id", nodeID,
		"library_id", node.LibraryID,
		"has_built_in_type", cmd.BuiltInType != nil,
		"has_archive_mode", cmd.ArchiveMode != nil,
		"has_view_meta", cmd.ViewMeta != nil,
	)
	return nil
}

func (u *NodeUseCase) Rename(ctx context.Context, nodeID uint64, cmd RenameNodeCommand) error {
	if err := u.ensureNodesConfigured(); err != nil {
		return err
	}

	newName := cmd.Name
	if nodeID == 0 || strings.TrimSpace(newName) == "" {
		return fmt.Errorf("%w: node id and name are required", ErrInvalidArgument)
	}

	node, err := u.GetNodeDetail(ctx, cmd.Actor, nodeID)
	if err != nil {
		return err
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, node.LibraryID); err != nil {
		return err
	}

	var ext *string
	if node.Type == domainnode.TypeFile && cmd.Ext != nil {
		trimmed := normalizeNodeExt(*cmd.Ext)
		if trimmed != "" && utf8.RuneCountInString(trimmed) > maxNodeExtLength {
			return fmt.Errorf("%w: ext is too long", ErrInvalidArgument)
		}
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
	slog.InfoContext(ctx, "node.renamed",
		"node_id", nodeID,
		"library_id", node.LibraryID,
		"dry_run", cmd.DryRun,
	)
	return nil
}

func (u *NodeUseCase) Move(ctx context.Context, cmd MoveNodeCommand) error {
	_, err := u.MoveBatch(ctx, MoveNodeBatchCommand{
		Actor:        cmd.Actor,
		LibraryID:    cmd.LibraryID,
		NewParentID:  cmd.NewParentID,
		BeforeNodeID: cmd.BeforeNodeID,
		Items: []MoveNodeBatchItemCommand{
			{
				NodeID: cmd.NodeID,
				Name:   cmd.Name,
			},
		},
		DryRun: cmd.DryRun,
	})
	return err
}

func (u *NodeUseCase) MoveBatch(ctx context.Context, cmd MoveNodeBatchCommand) (MoveNodeBatchResult, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return MoveNodeBatchResult{}, err
	}
	if cmd.LibraryID == 0 {
		return MoveNodeBatchResult{}, fmt.Errorf("%w: library id is required", ErrInvalidArgument)
	}
	if cmd.NewParentID == 0 {
		return MoveNodeBatchResult{}, fmt.Errorf("%w: new parent id is required", ErrInvalidArgument)
	}
	if len(cmd.Items) == 0 {
		return MoveNodeBatchResult{}, fmt.Errorf("%w: items are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return MoveNodeBatchResult{}, err
	}

	normalizedItems := normalizeMoveBatchItems(cmd.Items)
	if len(normalizedItems) == 0 {
		return MoveNodeBatchResult{}, fmt.Errorf("%w: items are required", ErrInvalidArgument)
	}

	result := MoveNodeBatchResult{}
	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		nodesByID := make(map[uint64]domainnode.Node, len(normalizedItems))
		selectedIDSet := make(map[uint64]struct{}, len(normalizedItems))
		for _, item := range normalizedItems {
			node, err := u.nodes.FindViewByNodeID(txCtx, item.NodeID)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return ErrNotFound
				}
				return err
			}
			if node.LibraryID != cmd.LibraryID {
				return fmt.Errorf("%w: node %d is not in library %d", ErrInvalidArgument, item.NodeID, cmd.LibraryID)
			}
			nodesByID[item.NodeID] = node
			selectedIDSet[item.NodeID] = struct{}{}
		}

		filteredItems := make([]MoveNodeBatchItemCommand, 0, len(normalizedItems))
		for _, item := range normalizedItems {
			ancestors, err := u.nodes.ListAncestors(txCtx, item.NodeID, cmd.LibraryID)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return ErrNotFound
				}
				return err
			}
			hasSelectedAncestor := false
			for _, ancestor := range ancestors {
				if ancestor.ID == item.NodeID {
					continue
				}
				if _, exists := selectedIDSet[ancestor.ID]; exists {
					hasSelectedAncestor = true
					break
				}
			}
			if !hasSelectedAncestor {
				filteredItems = append(filteredItems, item)
			}
		}
		if len(filteredItems) == 0 {
			return fmt.Errorf("%w: no movable items after normalization", ErrInvalidArgument)
		}

		targetParent, err := u.nodes.FindViewByNodeID(txCtx, cmd.NewParentID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}
		if targetParent.LibraryID != cmd.LibraryID {
			return fmt.Errorf("%w: target parent %d is not in library %d", ErrInvalidArgument, cmd.NewParentID, cmd.LibraryID)
		}
		if targetParent.ArchiveMode == 1 {
			return fmt.Errorf("%w: archive mode directory cannot be used as move target", ErrInvalidArgument)
		}

		parentAncestors, err := u.nodes.ListAncestors(txCtx, cmd.NewParentID, cmd.LibraryID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}
		parentAncestorSet := make(map[uint64]struct{}, len(parentAncestors))
		for _, ancestor := range parentAncestors {
			parentAncestorSet[ancestor.ID] = struct{}{}
		}

		movedIDSet := make(map[uint64]struct{}, len(filteredItems))
		for _, item := range filteredItems {
			if _, exists := parentAncestorSet[item.NodeID]; exists {
				return fmt.Errorf("%w: cannot move node under descendant", ErrInvalidArgument)
			}
			movedIDSet[item.NodeID] = struct{}{}
		}

		beforeNodeID := cmd.BeforeNodeID
		if _, exists := movedIDSet[beforeNodeID]; exists {
			beforeNodeID = 0
		}

		affectedParentSet := make(map[uint64]struct{}, len(filteredItems)+1)
		affectedParentSet[cmd.NewParentID] = struct{}{}
		movedNodeIDs := make([]uint64, 0, len(filteredItems))

		for _, item := range filteredItems {
			node := nodesByID[item.NodeID]
			if err := u.nodes.MoveNode(txCtx, repository.MoveNodeInput{
				LibraryID:    cmd.LibraryID,
				NodeID:       item.NodeID,
				NewParentID:  cmd.NewParentID,
				BeforeNodeID: beforeNodeID,
				Name:         item.Name,
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
			movedNodeIDs = append(movedNodeIDs, item.NodeID)
			affectedParentSet[node.ParentID] = struct{}{}
		}

		result = MoveNodeBatchResult{
			MovedCount:        len(movedNodeIDs),
			AffectedParentIDs: mapKeysSortedUint64(affectedParentSet),
			MovedNodeIDs:      movedNodeIDs,
		}
		return nil
	}); err != nil {
		return MoveNodeBatchResult{}, err
	}

	for _, item := range result.MovedNodeIDs {
		_ = u.RecordMoveIntent(ctx, MoveNodeCommand{
			Actor:        cmd.Actor,
			LibraryID:    cmd.LibraryID,
			NodeID:       item,
			NewParentID:  cmd.NewParentID,
			BeforeNodeID: cmd.BeforeNodeID,
			DryRun:       cmd.DryRun,
		})
	}
	_ = u.writeAudit(ctx, cmd.Actor, "node.move.batch", true, map[string]any{
		"library_id":          cmd.LibraryID,
		"new_parent_id":       cmd.NewParentID,
		"before_node_id":      cmd.BeforeNodeID,
		"moved_count":         result.MovedCount,
		"moved_node_ids":      result.MovedNodeIDs,
		"affected_parent_ids": result.AffectedParentIDs,
		"mode":                resolveMutationMode(cmd.DryRun),
		"dry_run":             cmd.DryRun,
	})
	slog.InfoContext(ctx, "node.moved.batch",
		"library_id", cmd.LibraryID,
		"new_parent_id", cmd.NewParentID,
		"before_node_id", cmd.BeforeNodeID,
		"moved_count", result.MovedCount,
		"dry_run", cmd.DryRun,
	)
	return result, nil
}

func normalizeMoveBatchItems(items []MoveNodeBatchItemCommand) []MoveNodeBatchItemCommand {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[uint64]struct{}, len(items))
	normalized := make([]MoveNodeBatchItemCommand, 0, len(items))
	for _, item := range items {
		if item.NodeID == 0 {
			continue
		}
		if _, exists := seen[item.NodeID]; exists {
			continue
		}
		seen[item.NodeID] = struct{}{}
		normalized = append(normalized, MoveNodeBatchItemCommand{
			NodeID: item.NodeID,
			Name:   strings.TrimSpace(item.Name),
		})
	}
	return normalized
}

func normalizeNodeExt(ext string) string {
	return strings.TrimPrefix(strings.TrimSpace(ext), ".")
}

func mapKeysSortedUint64(input map[uint64]struct{}) []uint64 {
	if len(input) == 0 {
		return []uint64{}
	}
	output := make([]uint64, 0, len(input))
	for item := range input {
		output = append(output, item)
	}
	sort.Slice(output, func(i, j int) bool {
		return output[i] < output[j]
	})
	return output
}

func (u *NodeUseCase) SortComicChildrenByName(ctx context.Context, cmd SortComicChildrenCommand) error {
	if cmd.NodeID == 0 {
		return fmt.Errorf("%w: node id is required", ErrInvalidArgument)
	}

	node, err := u.GetNodeDetail(ctx, cmd.Actor, cmd.NodeID)
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
	slog.InfoContext(ctx, "node.comic_children.sorted_by_name",
		"node_id", cmd.NodeID,
		"library_id", node.LibraryID,
	)
	return nil
}

func (u *NodeUseCase) BatchSetArchiveChildrenBuiltInType(
	ctx context.Context,
	cmd BatchSetArchiveChildrenBuiltInTypeCommand,
) (BatchSetArchiveChildrenBuiltInTypeResult, error) {
	if cmd.NodeID == 0 {
		return BatchSetArchiveChildrenBuiltInTypeResult{}, fmt.Errorf("%w: node id is required", ErrInvalidArgument)
	}

	node, err := u.GetNodeDetail(ctx, cmd.Actor, cmd.NodeID)
	if err != nil {
		return BatchSetArchiveChildrenBuiltInTypeResult{}, err
	}
	if node.Type != domainnode.TypeDirectory {
		return BatchSetArchiveChildrenBuiltInTypeResult{}, fmt.Errorf("%w: target node must be a directory", ErrInvalidArgument)
	}
	if node.ArchiveMode != 1 {
		return BatchSetArchiveChildrenBuiltInTypeResult{}, fmt.Errorf("%w: target directory must enable archive mode", ErrInvalidArgument)
	}

	builtInType := strings.ToUpper(strings.TrimSpace(node.BuiltInType))
	if builtInType == "" {
		builtInType = "DEF"
	}
	if builtInType == "DEF" {
		return BatchSetArchiveChildrenBuiltInTypeResult{}, fmt.Errorf("%w: target directory must set non-DEF built-in type", ErrInvalidArgument)
	}

	if err := u.AuthorizeMutation(ctx, cmd.Actor, node.LibraryID); err != nil {
		return BatchSetArchiveChildrenBuiltInTypeResult{}, err
	}

	children, err := u.nodes.ListDirectChildren(ctx, cmd.NodeID, node.LibraryID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return BatchSetArchiveChildrenBuiltInTypeResult{}, ErrNotFound
		}
		return BatchSetArchiveChildrenBuiltInTypeResult{}, err
	}

	totalChildren := len(children)
	directories := lo.Filter(children, func(child domainnode.Node, _ int) bool {
		return child.Type == domainnode.TypeDirectory
	})
	dirChildren := len(directories)

	var updatedCount int64
	if dirChildren > 0 {
		if err := u.withinTx(ctx, func(txCtx context.Context) error {
			rows, updateErr := u.nodes.BatchSetDirectChildDirectoriesBuiltInType(
				txCtx,
				cmd.NodeID,
				node.LibraryID,
				builtInType,
				time.Now().UTC(),
			)
			if updateErr != nil {
				if errors.Is(updateErr, repository.ErrInvalidState) {
					return fmt.Errorf("%w: invalid archive batch set request", ErrInvalidArgument)
				}
				return updateErr
			}
			updatedCount = rows
			return nil
		}); err != nil {
			return BatchSetArchiveChildrenBuiltInTypeResult{}, err
		}

		if normalizedBuiltIn := normalizeArchiveCardBuiltInType(builtInType); normalizedBuiltIn != "" {
			archiveItems := lo.Map(directories, func(child domainnode.Node, _ int) ArchiveCardItem {
				return ArchiveCardItem{
					ID:       child.ID,
					ViewMeta: child.ViewMeta,
				}
			})
			_ = u.warmupArchiveCoverMetaForNodes(ctx, node.LibraryID, normalizedBuiltIn, archiveItems)
		}
	}

	result := BatchSetArchiveChildrenBuiltInTypeResult{
		NodeID:        cmd.NodeID,
		LibraryID:     node.LibraryID,
		BuiltInType:   builtInType,
		TotalChildren: totalChildren,
		DirChildren:   dirChildren,
		UpdatedCount:  int(updatedCount),
	}

	_ = u.writeAudit(ctx, cmd.Actor, "node.batch_set_archive_children_built_in_type", true, map[string]any{
		"node_id":        cmd.NodeID,
		"library_id":     node.LibraryID,
		"built_in_type":  builtInType,
		"total_children": totalChildren,
		"dir_children":   dirChildren,
		"updated_count":  updatedCount,
	})
	slog.InfoContext(ctx, "node.archive_children.built_in_type_batch_set",
		"node_id", cmd.NodeID,
		"library_id", node.LibraryID,
		"built_in_type", builtInType,
		"total_children", totalChildren,
		"dir_children", dirChildren,
		"updated_count", updatedCount,
	)
	return result, nil
}

func (u *NodeUseCase) DeleteNodeAndChildren(ctx context.Context, cmd DeleteNodeTreeCommand) (bool, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return false, err
	}

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
	slog.InfoContext(ctx, "node.tree.deleted",
		"node_id", cmd.NodeID,
		"library_id", cmd.LibraryID,
		"deleted_nodes", deleteResult.DeletedNodeCount,
		"file_nodes", deleteResult.FileNodeCount,
		"dry_run", cmd.DryRun,
	)
	return true, nil
}

func (u *NodeUseCase) GetRecycleBinItems(ctx context.Context, libraryID uint64) ([]domainnode.RecycleItem, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return nil, err
	}

	if libraryID == 0 {
		slog.WarnContext(ctx, "node.recycle.invalid_argument", "reason", "library_id_missing")
		return nil, fmt.Errorf("%w: library id is required", ErrInvalidArgument)
	}

	rows, err := u.nodes.ListDeletedNodes(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []domainnode.RecycleItem{}, nil
	}

	deletedSet := lo.SliceToMap(rows, func(item domainnode.RecycleItem) (uint64, struct{}) {
		return item.ID, struct{}{}
	})
	childrenByParent := make(map[uint64][]uint64, len(rows))
	for _, item := range rows {
		if item.ParentID == 0 {
			continue
		}
		if _, ok := deletedSet[item.ParentID]; !ok {
			continue
		}
		childrenByParent[item.ParentID] = append(childrenByParent[item.ParentID], item.ID)
	}

	topLevel := lo.Filter(rows, func(item domainnode.RecycleItem, _ int) bool {
		if item.ParentID == 0 {
			return true
		}
		_, ok := deletedSet[item.ParentID]
		return !ok
	})

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
	slog.DebugContext(ctx, "node.recycle.listed",
		"library_id", libraryID,
		"result_count", len(topLevel),
		"raw_deleted_count", len(rows),
	)
	return topLevel, nil
}

func (u *NodeUseCase) RestoreNodeAndChildren(ctx context.Context, cmd RestoreNodeTreeCommand) (bool, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return false, err
	}

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
	slog.InfoContext(ctx, "node.tree.restored",
		"node_id", cmd.NodeID,
		"library_id", cmd.LibraryID,
		"dry_run", cmd.DryRun,
		"result", ok,
	)
	return ok, nil
}

func (u *NodeUseCase) HardDeleteNodeAndChildren(ctx context.Context, cmd HardDeleteNodeTreeCommand) (bool, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return false, err
	}

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
	slog.InfoContext(ctx, "node.tree.hard_deleted",
		"node_id", cmd.NodeID,
		"library_id", cmd.LibraryID,
		"dry_run", cmd.DryRun,
		"result", ok,
	)
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

func (u *NodeUseCase) ensureNodesConfigured() error {
	if u == nil || u.nodes == nil {
		return errNodeRepositoryNotConfigured
	}
	return nil
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

	return lo.Uniq(lo.Filter(values, func(value uint64, _ int) bool {
		return value > 0
	}))
}
