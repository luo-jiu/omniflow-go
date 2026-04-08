package usecase

import (
	"context"
	"errors"
	"fmt"
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
}

type UpdateNodeCommand struct {
	Actor       actor.Actor
	LibraryID   uint64
	BuiltInType *string
	ArchiveMode *int
}

type RenameNodeCommand struct {
	Actor     actor.Actor
	LibraryID uint64
	Name      string
}

type MoveNodeCommand struct {
	Actor        actor.Actor
	LibraryID    uint64
	NodeID       uint64
	NewParentID  uint64
	BeforeNodeID uint64
	Name         string
}

type DeleteNodeTreeCommand struct {
	Actor     actor.Actor
	LibraryID uint64
	NodeID    uint64
}

type NodeUseCase struct {
	nodes      *repository.NodeRepository
	authorizer authz.Authorizer
	auditLog   audit.Sink
}

func NewNodeUseCase(
	nodes *repository.NodeRepository,
	authorizer authz.Authorizer,
	auditLog audit.Sink,
) *NodeUseCase {
	return &NodeUseCase{
		nodes:      nodes,
		authorizer: authorizer,
		auditLog:   auditLog,
	}
}

const (
	defaultStorageProvider = "MINIO"
	defaultStorageBucket   = "my-bucket"
)

func (u *NodeUseCase) Create(ctx context.Context, cmd CreateNodeCommand) (domainnode.Node, error) {
	name := strings.TrimSpace(cmd.Name)
	if cmd.LibraryID == 0 || name == "" {
		return domainnode.Node{}, fmt.Errorf("%w: library id and name are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return domainnode.Node{}, err
	}

	created, err := u.nodes.CreateNode(ctx, repository.CreateNodeInput{
		Name:            name,
		Type:            cmd.Type,
		ParentID:        cmd.ParentID,
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
			return domainnode.Node{}, ErrNotFound
		}
		if errors.Is(err, repository.ErrConflict) {
			return domainnode.Node{}, ErrConflict
		}
		if errors.Is(err, repository.ErrInvalidState) {
			return domainnode.Node{}, fmt.Errorf("%w: invalid node create request", ErrInvalidArgument)
		}
		return domainnode.Node{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "node.create", true, map[string]any{
		"node_id":    created.ID,
		"library_id": created.LibraryID,
		"parent_id":  created.ParentID,
		"type":       created.Type,
		"name":       created.Name,
	})
	return created, nil
}

func (u *NodeUseCase) GetAllDescendants(ctx context.Context, nodeID, libraryID uint64) ([]domainnode.Node, error) {
	if libraryID == 0 || nodeID == 0 {
		return nil, fmt.Errorf("%w: node id and library id are required", ErrInvalidArgument)
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

func (u *NodeUseCase) GetDirectChildren(ctx context.Context, nodeID, libraryID uint64) ([]domainnode.Node, error) {
	if libraryID == 0 || nodeID == 0 {
		return nil, fmt.Errorf("%w: node id and library id are required", ErrInvalidArgument)
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

func (u *NodeUseCase) GetAncestors(ctx context.Context, nodeID, libraryID uint64) ([]NodePath, error) {
	if libraryID == 0 || nodeID == 0 {
		return nil, fmt.Errorf("%w: node id and library id are required", ErrInvalidArgument)
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

func (u *NodeUseCase) GetFullPath(ctx context.Context, nodeID, libraryID uint64) (string, error) {
	ancestors, err := u.GetAncestors(ctx, nodeID, libraryID)
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

func (u *NodeUseCase) GetLibraryRootNodeID(ctx context.Context, libraryID uint64) (uint64, error) {
	if libraryID == 0 {
		return 0, fmt.Errorf("%w: library id is required", ErrInvalidArgument)
	}

	rootNodeID, err := u.nodes.EnsureLibraryRootNodeID(ctx, libraryID)
	if err != nil {
		return 0, err
	}
	return rootNodeID, nil
}

func (u *NodeUseCase) Update(ctx context.Context, nodeID uint64, cmd UpdateNodeCommand) error {
	if nodeID == 0 || cmd.LibraryID == 0 {
		return fmt.Errorf("%w: node id and library id are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return err
	}
	if _, err := u.findNodeView(ctx, nodeID, cmd.LibraryID); err != nil {
		return err
	}

	updates := map[string]any{}
	if cmd.BuiltInType != nil {
		updates["built_in_type"] = strings.TrimSpace(*cmd.BuiltInType)
	}
	if cmd.ArchiveMode != nil {
		if *cmd.ArchiveMode != 0 && *cmd.ArchiveMode != 1 {
			return fmt.Errorf("%w: archive mode only supports 0 or 1", ErrInvalidArgument)
		}
		updates["archive_mode"] = (*cmd.ArchiveMode == 1)
	}
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now().UTC()

	ok, err := u.nodes.UpdateNodeFields(ctx, nodeID, cmd.LibraryID, updates)
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
		"library_id": cmd.LibraryID,
	})
	return nil
}

func (u *NodeUseCase) Rename(ctx context.Context, nodeID uint64, cmd RenameNodeCommand) error {
	newName := strings.TrimSpace(cmd.Name)
	if nodeID == 0 || cmd.LibraryID == 0 || newName == "" {
		return fmt.Errorf("%w: node id, library id and name are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return err
	}

	if err := u.nodes.RenameNode(ctx, nodeID, cmd.LibraryID, newName, time.Now().UTC()); err != nil {
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

	_ = u.writeAudit(ctx, cmd.Actor, "node.rename", true, map[string]any{
		"node_id":    nodeID,
		"library_id": cmd.LibraryID,
		"name":       newName,
	})
	return nil
}

func (u *NodeUseCase) Move(ctx context.Context, cmd MoveNodeCommand) error {
	if cmd.LibraryID == 0 || cmd.NodeID == 0 {
		return fmt.Errorf("%w: library id and node id are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return err
	}

	if err := u.nodes.MoveNode(ctx, repository.MoveNodeInput{
		LibraryID:    cmd.LibraryID,
		NodeID:       cmd.NodeID,
		NewParentID:  cmd.NewParentID,
		BeforeNodeID: cmd.BeforeNodeID,
		Name:         strings.TrimSpace(cmd.Name),
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

	_ = u.RecordMoveIntent(ctx, cmd)
	_ = u.writeAudit(ctx, cmd.Actor, "node.move", true, map[string]any{
		"node_id":        cmd.NodeID,
		"library_id":     cmd.LibraryID,
		"new_parent_id":  cmd.NewParentID,
		"before_node_id": cmd.BeforeNodeID,
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

	deleteResult, err := u.nodes.DeleteTree(ctx, cmd.NodeID, cmd.LibraryID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, ErrNotFound
		}
		return false, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "node.delete_tree", true, map[string]any{
		"node_id":       cmd.NodeID,
		"library_id":    cmd.LibraryID,
		"deleted_nodes": deleteResult.DeletedNodeCount,
		"file_nodes":    deleteResult.FileNodeCount,
	})
	return true, nil
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

func (u *NodeUseCase) AuthorizeMutation(ctx context.Context, principal actor.Actor, libraryID uint64) error {
	if u.authorizer == nil {
		return nil
	}

	return u.authorizer.Authorize(ctx, principal, authz.Resource{
		Kind: "library",
		ID:   fmt.Sprintf("%d", libraryID),
	}, authz.ActionWrite)
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
