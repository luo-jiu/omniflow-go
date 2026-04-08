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

	"gorm.io/gorm"
)

type ListChildrenQuery struct {
	Actor     actor.Actor
	LibraryID uint64
	NodeID    uint64
}

type NodePath struct {
	ID    uint64
	Name  string
	Depth int
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

	db, err := dbFromRepository(u.nodes)
	if err != nil {
		return domainnode.Node{}, err
	}

	var created nodeRecord
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if cmd.ParentID > 0 {
			var parent nodeRecord
			if err := tx.First(&parent, "id = ? AND library_id = ?", cmd.ParentID, cmd.LibraryID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrNotFound
				}
				return err
			}
			if parent.NodeType != nodeTypeDirectory {
				return fmt.Errorf("%w: parent node must be a directory", ErrInvalidArgument)
			}
		}

		if err := u.checkDuplicateName(tx, name, cmd.ParentID, cmd.LibraryID, 0); err != nil {
			return err
		}

		query := tx.Model(&nodeRecord{}).
			Select("COALESCE(MAX(sort_order), 0)").
			Where("library_id = ?", cmd.LibraryID).
			Where("deleted_at IS NULL")
		query = applyParentFilter(query, cmd.ParentID)

		var maxSort int
		if err := query.Scan(&maxSort).Error; err != nil {
			return err
		}

		var ext *string
		if strings.TrimSpace(cmd.Ext) != "" {
			trimmedExt := strings.TrimSpace(cmd.Ext)
			ext = &trimmedExt
		}
		if cmd.Type == domainnode.TypeDirectory {
			ext = nil
		}

		record := nodeRecord{
			Name:      name,
			Ext:       ext,
			NodeType:  nodeTypeToCode(cmd.Type),
			ParentID:  normalizeParentID(cmd.ParentID),
			LibraryID: cmd.LibraryID,
			BuiltIn:   "DEF",
			Archive:   false,
			SortOrder: maxSort + 15,
		}

		if err := tx.Create(&record).Error; err != nil {
			return err
		}

		if cmd.Type == domainnode.TypeFile {
			storageKey := strings.TrimSpace(cmd.StorageKey)
			if storageKey == "" {
				return fmt.Errorf("%w: storage key is required for file node", ErrInvalidArgument)
			}
			if cmd.FileSize < 0 {
				return fmt.Errorf("%w: file size must be >= 0", ErrInvalidArgument)
			}

			storageObj := storageObjectRecord{
				LibraryID:     cmd.LibraryID,
				Provider:      defaultStorageProvider,
				Bucket:        defaultStorageBucket,
				ObjectKey:     storageKey,
				ContentLength: cmd.FileSize,
				ContentType:   strings.TrimSpace(cmd.MIMEType),
			}
			if err := tx.Create(&storageObj).Error; err != nil {
				return err
			}

			fileRec := nodeFileRecord{
				FileID:          record.ID,
				LibraryID:       cmd.LibraryID,
				StorageObjectID: storageObj.ID,
				MIMEType:        strings.TrimSpace(cmd.MIMEType),
				FileSize:        cmd.FileSize,
			}
			if err := tx.Create(&fileRec).Error; err != nil {
				return err
			}

			record.MIMEType = fileRec.MIMEType
			record.FileSize = fileRec.FileSize
			record.StorageKey = storageObj.ObjectKey
		}

		created = record
		return nil
	})
	if err != nil {
		return domainnode.Node{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "node.create", true, map[string]any{
		"node_id":    created.ID,
		"library_id": created.LibraryID,
		"parent_id":  parentIDValue(created.ParentID),
		"type":       created.NodeType,
		"name":       created.Name,
	})
	return created.toDomain(), nil
}

func (u *NodeUseCase) GetAllDescendants(ctx context.Context, nodeID, libraryID uint64) ([]domainnode.Node, error) {
	if libraryID == 0 || nodeID == 0 {
		return nil, fmt.Errorf("%w: node id and library id are required", ErrInvalidArgument)
	}

	db, err := dbFromRepository(u.nodes)
	if err != nil {
		return nil, err
	}

	var rows []nodeRecord
	query := `
WITH RECURSIVE tree AS (
    SELECT id, parent_id, 0 AS depth
    FROM nodes
    WHERE id = ? AND library_id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT n.id, n.parent_id, tree.depth + 1
    FROM nodes n
    JOIN tree ON n.parent_id = tree.id
    WHERE n.library_id = ? AND n.deleted_at IS NULL
)
SELECT
    v.id,
    v.name,
    v.parent_id,
    v.node_type,
    v.library_id,
    v.ext,
    v.mime_type,
    v.file_size,
    v.storage_key,
    v.built_in_type,
    v.archive_mode,
    v.sort_order
FROM tree
JOIN v_live_nodes v ON v.id = tree.id AND v.library_id = ?
ORDER BY tree.depth ASC, v.sort_order ASC, v.id ASC`
	if err := db.WithContext(ctx).Raw(query, nodeID, libraryID, libraryID, libraryID).Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]domainnode.Node, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, nil
}

func (u *NodeUseCase) GetDirectChildren(ctx context.Context, nodeID, libraryID uint64) ([]domainnode.Node, error) {
	if libraryID == 0 || nodeID == 0 {
		return nil, fmt.Errorf("%w: node id and library id are required", ErrInvalidArgument)
	}

	db, err := dbFromRepository(u.nodes)
	if err != nil {
		return nil, err
	}

	var rows []nodeRecord
	if err := db.WithContext(ctx).
		Table("v_live_nodes").
		Select("id, name, parent_id, node_type, library_id, ext, mime_type, file_size, storage_key, built_in_type, archive_mode, sort_order").
		Where("library_id = ? AND parent_id = ?", libraryID, nodeID).
		Order("sort_order ASC").
		Order("id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]domainnode.Node, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, nil
}

func (u *NodeUseCase) GetAncestors(ctx context.Context, nodeID, libraryID uint64) ([]NodePath, error) {
	if libraryID == 0 || nodeID == 0 {
		return nil, fmt.Errorf("%w: node id and library id are required", ErrInvalidArgument)
	}

	db, err := dbFromRepository(u.nodes)
	if err != nil {
		return nil, err
	}

	var rows []NodePath
	query := `
WITH RECURSIVE ancestors AS (
    SELECT id, name, parent_id, 0 AS depth
    FROM nodes
    WHERE id = ? AND library_id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT p.id, p.name, p.parent_id, ancestors.depth + 1
    FROM nodes p
    JOIN ancestors ON ancestors.parent_id = p.id
    WHERE p.library_id = ? AND p.deleted_at IS NULL
)
SELECT id, name, depth
FROM ancestors
ORDER BY depth DESC`
	if err := db.WithContext(ctx).Raw(query, nodeID, libraryID, libraryID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
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

func (u *NodeUseCase) Update(ctx context.Context, nodeID uint64, cmd UpdateNodeCommand) error {
	if nodeID == 0 || cmd.LibraryID == 0 {
		return fmt.Errorf("%w: node id and library id are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return err
	}
	if _, err := u.findNode(ctx, nodeID, cmd.LibraryID); err != nil {
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

	db, err := dbFromRepository(u.nodes)
	if err != nil {
		return err
	}
	result := db.WithContext(ctx).
		Model(&nodeRecord{}).
		Where("id = ? AND library_id = ?", nodeID, cmd.LibraryID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
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

	db, err := dbFromRepository(u.nodes)
	if err != nil {
		return err
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		node, err := u.findNodeWithDB(ctx, tx, nodeID, cmd.LibraryID)
		if err != nil {
			return err
		}
		if err := u.checkDuplicateName(tx, newName, parentIDValue(node.ParentID), node.LibraryID, node.ID); err != nil {
			return err
		}
		if err := tx.Model(&nodeRecord{}).
			Where("id = ? AND library_id = ?", nodeID, cmd.LibraryID).
			Updates(map[string]any{
				"name":       newName,
				"updated_at": time.Now().UTC(),
			}).Error; err != nil {
			return err
		}

		_ = u.writeAudit(ctx, cmd.Actor, "node.rename", true, map[string]any{
			"node_id":    nodeID,
			"library_id": cmd.LibraryID,
			"name":       newName,
		})
		return nil
	})
}

func (u *NodeUseCase) Move(ctx context.Context, cmd MoveNodeCommand) error {
	if cmd.LibraryID == 0 || cmd.NodeID == 0 {
		return fmt.Errorf("%w: library id and node id are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return err
	}

	db, err := dbFromRepository(u.nodes)
	if err != nil {
		return err
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		node, err := u.findNodeWithDB(ctx, tx, cmd.NodeID, cmd.LibraryID)
		if err != nil {
			return err
		}

		if cmd.NewParentID == cmd.NodeID {
			return fmt.Errorf("%w: node cannot be moved under itself", ErrInvalidArgument)
		}

		if cmd.NewParentID > 0 {
			newParent, err := u.findNodeWithDB(ctx, tx, cmd.NewParentID, cmd.LibraryID)
			if err != nil {
				return err
			}
			if newParent.NodeType != nodeTypeDirectory {
				return fmt.Errorf("%w: target parent must be a directory", ErrInvalidArgument)
			}
		}

		if cmd.NewParentID > 0 {
			isDescendant, err := u.isDescendant(tx, cmd.NodeID, cmd.NewParentID, cmd.LibraryID)
			if err != nil {
				return err
			}
			if isDescendant {
				return fmt.Errorf("%w: cannot move node under a descendant", ErrInvalidArgument)
			}
		}

		name := strings.TrimSpace(cmd.Name)
		if name == "" {
			name = node.Name
		}
		if err := u.checkDuplicateName(tx, name, cmd.NewParentID, cmd.LibraryID, cmd.NodeID); err != nil {
			return err
		}

		var newOrder int
		if cmd.BeforeNodeID > 0 {
			beforeNode, err := u.findNodeWithDB(ctx, tx, cmd.BeforeNodeID, cmd.LibraryID)
			if err != nil {
				return err
			}
			if parentIDValue(beforeNode.ParentID) != cmd.NewParentID {
				return fmt.Errorf("%w: before node is not under target parent", ErrInvalidArgument)
			}

			shiftQuery := tx.Model(&nodeRecord{}).
				Where("library_id = ? AND sort_order >= ?", cmd.LibraryID, beforeNode.SortOrder).
				Where("deleted_at IS NULL")
			shiftQuery = applyParentFilter(shiftQuery, cmd.NewParentID)
			if err := shiftQuery.Update("sort_order", gorm.Expr("sort_order + 1")).Error; err != nil {
				return err
			}
			newOrder = beforeNode.SortOrder
		} else {
			maxSortQuery := tx.Model(&nodeRecord{}).
				Select("COALESCE(MAX(sort_order), 0)").
				Where("library_id = ?", cmd.LibraryID).
				Where("deleted_at IS NULL")
			maxSortQuery = applyParentFilter(maxSortQuery, cmd.NewParentID)

			var maxSort int
			if err := maxSortQuery.Scan(&maxSort).Error; err != nil {
				return err
			}
			newOrder = maxSort + 15
		}

		if err := tx.Model(&nodeRecord{}).
			Where("id = ? AND library_id = ?", cmd.NodeID, cmd.LibraryID).
			Updates(map[string]any{
				"parent_id":  normalizeParentID(cmd.NewParentID),
				"sort_order": newOrder,
				"updated_at": time.Now().UTC(),
			}).Error; err != nil {
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
	})
}

func (u *NodeUseCase) DeleteNodeAndChildren(ctx context.Context, cmd DeleteNodeTreeCommand) (bool, error) {
	if cmd.LibraryID == 0 || cmd.NodeID == 0 {
		return false, fmt.Errorf("%w: library id and node id are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return false, err
	}

	db, err := dbFromRepository(u.nodes)
	if err != nil {
		return false, err
	}

	var deletedNodeCount int
	var fileCount int64
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var descendantIDs []uint64
		descendantQuery := `
WITH RECURSIVE sub AS (
    SELECT id
    FROM nodes
    WHERE id = ? AND library_id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT n.id
    FROM nodes n
    JOIN sub s ON n.parent_id = s.id
    WHERE n.library_id = ? AND n.deleted_at IS NULL
)
SELECT id FROM sub`
		if err := tx.Raw(descendantQuery, cmd.NodeID, cmd.LibraryID, cmd.LibraryID).Scan(&descendantIDs).Error; err != nil {
			return err
		}
		if len(descendantIDs) == 0 {
			return nil
		}

		if err := tx.Table("v_live_nodes").
			Where("library_id = ? AND id IN ?", cmd.LibraryID, descendantIDs).
			Where("node_type = ? AND storage_key IS NOT NULL AND storage_key <> ''", nodeTypeFile).
			Count(&fileCount).Error; err != nil {
			return err
		}
		deletedNodeCount = len(descendantIDs)

		if err := tx.Where("library_id = ? AND id IN ?", cmd.LibraryID, descendantIDs).Delete(&nodeRecord{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return false, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "node.delete_tree", true, map[string]any{
		"node_id":       cmd.NodeID,
		"library_id":    cmd.LibraryID,
		"deleted_nodes": deletedNodeCount,
		"file_nodes":    fileCount,
	})
	return true, nil
}

func (u *NodeUseCase) findNode(ctx context.Context, nodeID, libraryID uint64) (nodeRecord, error) {
	db, err := dbFromRepository(u.nodes)
	if err != nil {
		return nodeRecord{}, err
	}
	return u.findNodeWithDB(ctx, db.WithContext(ctx), nodeID, libraryID)
}

func (u *NodeUseCase) findNodeWithDB(_ context.Context, db *gorm.DB, nodeID, libraryID uint64) (nodeRecord, error) {
	var node nodeRecord
	if err := db.First(&node, "id = ? AND library_id = ?", nodeID, libraryID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nodeRecord{}, ErrNotFound
		}
		return nodeRecord{}, err
	}
	return node, nil
}

func (u *NodeUseCase) findNodeView(ctx context.Context, nodeID, libraryID uint64) (nodeRecord, error) {
	db, err := dbFromRepository(u.nodes)
	if err != nil {
		return nodeRecord{}, err
	}

	var row nodeRecord
	if err := db.WithContext(ctx).
		Table("v_live_nodes").
		Select("id, name, parent_id, node_type, library_id, ext, mime_type, file_size, storage_key, built_in_type, archive_mode, sort_order").
		Where("id = ? AND library_id = ?", nodeID, libraryID).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nodeRecord{}, ErrNotFound
		}
		return nodeRecord{}, err
	}
	return row, nil
}

func (u *NodeUseCase) checkDuplicateName(tx *gorm.DB, name string, parentID, libraryID, excludeID uint64) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: node name is required", ErrInvalidArgument)
	}

	query := tx.Model(&nodeRecord{}).
		Where("name = ? AND library_id = ?", name, libraryID).
		Where("deleted_at IS NULL")
	query = applyParentFilter(query, parentID)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrConflict
	}
	return nil
}

func (u *NodeUseCase) isDescendant(tx *gorm.DB, nodeID, targetID, libraryID uint64) (bool, error) {
	var count int64
	query := `
WITH RECURSIVE sub AS (
    SELECT id
    FROM nodes
    WHERE id = ? AND library_id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT n.id
    FROM nodes n
    JOIN sub s ON n.parent_id = s.id
    WHERE n.library_id = ? AND n.deleted_at IS NULL
)
SELECT COUNT(1)
FROM sub
WHERE id = ?`
	if err := tx.Raw(query, nodeID, libraryID, libraryID, targetID).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func normalizeParentID(parentID uint64) *uint64 {
	if parentID == 0 {
		return nil
	}
	value := parentID
	return &value
}

func parentIDValue(parentID *uint64) uint64 {
	if parentID == nil {
		return 0
	}
	return *parentID
}

func applyParentFilter(query *gorm.DB, parentID uint64) *gorm.DB {
	if parentID == 0 {
		return query.Where("parent_id IS NULL")
	}
	return query.Where("parent_id = ?", parentID)
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
