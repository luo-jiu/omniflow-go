package repository

import (
	"context"
	"errors"
	"time"

	pgmodel "omniflow-go/internal/repository/postgres/model"

	"gorm.io/gorm"
)

const defaultRootNodeName = "根目录"

// EnsureLibraryRootNodeID 返回资料库根节点 ID；若不存在则自动创建。
func (r *NodeRepository) EnsureLibraryRootNodeID(ctx context.Context, libraryID uint64) (uint64, error) {
	root, err := r.findLibraryRootNode(ctx, libraryID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		root = &pgmodel.Node{
			Name:        defaultRootNodeName,
			BuiltInType: "DEF",
			NodeType:    nodeTypeDirectory,
			ArchiveMode: false,
			ViewMeta:    "{}",
			SortOrder:   0,
			ParentID:    nil,
			LibraryID:   toPGInt64(libraryID),
		}
		if createErr := r.dbWithContext(ctx).Create(root).Error; createErr != nil {
			return 0, createErr
		}
	}

	rootID := toDomainUint64(root.ID)
	if _, repairErr := r.repairParentReferences(ctx, libraryID, rootID); repairErr != nil {
		return 0, repairErr
	}

	return rootID, nil
}

func (r *NodeRepository) findLibraryRootNode(ctx context.Context, libraryID uint64) (*pgmodel.Node, error) {
	var root pgmodel.Node
	if err := r.dbWithContext(ctx).
		Model(&pgmodel.Node{}).
		Where(
			"library_id = ? AND node_type = ? AND deleted_at IS NULL AND (parent_id IS NULL OR parent_id = 0)",
			toPGInt64(libraryID),
			nodeTypeDirectory,
		).
		Order("id ASC").
		First(&root).Error; err != nil {
		return nil, err
	}
	return &root, nil
}

func (r *NodeRepository) repairParentReferences(ctx context.Context, libraryID, rootNodeID uint64) (bool, error) {
	var rows []pgmodel.Node
	if err := r.dbWithContext(ctx).
		Unscoped().
		Model(&pgmodel.Node{}).
		Where("library_id = ?", toPGInt64(libraryID)).
		Find(&rows).Error; err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, nil
	}

	nodesByID := make(map[int64]*pgmodel.Node, len(rows))
	for i := range rows {
		nodesByID[rows[i].ID] = &rows[i]
	}

	changed := false
	now := time.Now().UTC()
	for i := range rows {
		node := &rows[i]
		nodeID := toDomainUint64(node.ID)
		currentParentID := parentIDValue(node.ParentID)
		targetParentID := currentParentID

		if nodeID == rootNodeID {
			targetParentID = 0
		} else if currentParentID == 0 {
			targetParentID = rootNodeID
		} else {
			parent, ok := nodesByID[toPGInt64(currentParentID)]
			if !ok {
				targetParentID = rootNodeID
			} else if parent.NodeType != nodeTypeDirectory {
				targetParentID = rootNodeID
			} else if !node.DeletedAt.Valid && parent.DeletedAt.Valid {
				targetParentID = rootNodeID
			}
		}

		if targetParentID == nodeID {
			targetParentID = rootNodeID
		}

		if targetParentID == currentParentID {
			continue
		}

		result := r.dbWithContext(ctx).
			Unscoped().
			Model(&pgmodel.Node{}).
			Where("id = ? AND library_id = ?", node.ID, toPGInt64(libraryID)).
			Updates(map[string]any{
				"parent_id":  normalizePGParentID(targetParentID),
				"updated_at": now,
			})
		if result.Error != nil {
			return false, result.Error
		}
		if result.RowsAffected > 0 {
			changed = true
		}
	}

	return changed, nil
}
