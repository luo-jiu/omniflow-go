package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	domainnode "omniflow-go/internal/domain/node"

	"gorm.io/gorm"
)

type CreateNodeInput struct {
	Name            string
	Type            domainnode.Type
	ParentID        uint64
	LibraryID       uint64
	Ext             string
	MIMEType        string
	FileSize        int64
	StorageKey      string
	BuiltInType     string
	ArchiveMode     bool
	StorageProvider string
	StorageBucket   string
}

type MoveNodeInput struct {
	LibraryID    uint64
	NodeID       uint64
	NewParentID  uint64
	BeforeNodeID uint64
	Name         string
	UpdatedAt    time.Time
}

// CreateNode 创建节点并在文件类型时补齐对象存储关联。
func (r *NodeRepository) CreateNode(ctx context.Context, input CreateNodeInput) (domainnode.Node, error) {
	name := strings.TrimSpace(input.Name)

	var createdID uint64
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if input.ParentID > 0 {
			parent, err := r.WithTx(tx).findNodeModel(ctx, input.ParentID, input.LibraryID)
			if err != nil {
				return err
			}
			if parent.NodeType != nodeTypeDirectory {
				return fmt.Errorf("%w: parent node must be directory", ErrInvalidState)
			}
		}

		duplicate, err := r.WithTx(tx).hasDuplicateName(ctx, name, input.ParentID, input.LibraryID, 0)
		if err != nil {
			return err
		}
		if duplicate {
			return ErrConflict
		}

		maxSort, err := r.WithTx(tx).maxSortOrder(ctx, input.LibraryID, input.ParentID)
		if err != nil {
			return err
		}

		var ext *string
		if input.Type == domainnode.TypeFile {
			trimmedExt := strings.TrimSpace(input.Ext)
			if trimmedExt != "" {
				ext = &trimmedExt
			}
			if strings.TrimSpace(input.StorageKey) == "" {
				return fmt.Errorf("%w: storage key is required for file node", ErrInvalidState)
			}
			if input.FileSize < 0 {
				return fmt.Errorf("%w: file size must be >= 0", ErrInvalidState)
			}
		}

		row := nodeModel{
			Name:      name,
			Ext:       ext,
			BuiltIn:   strings.TrimSpace(input.BuiltInType),
			NodeType:  nodeTypeCode(input.Type),
			Archive:   input.ArchiveMode,
			SortOrder: maxSort + 15,
			ParentID:  normalizeParentID(input.ParentID),
			LibraryID: input.LibraryID,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		createdID = row.ID

		if input.Type != domainnode.TypeFile {
			return nil
		}

		storageRow := storageObjectModel{
			LibraryID:     input.LibraryID,
			Provider:      strings.TrimSpace(input.StorageProvider),
			Bucket:        strings.TrimSpace(input.StorageBucket),
			ObjectKey:     strings.TrimSpace(input.StorageKey),
			ContentLength: input.FileSize,
			ContentType:   strings.TrimSpace(input.MIMEType),
		}
		if err := tx.Create(&storageRow).Error; err != nil {
			return err
		}

		fileRow := nodeFileModel{
			FileID:          row.ID,
			LibraryID:       input.LibraryID,
			StorageObjectID: storageRow.ID,
			MIMEType:        strings.TrimSpace(input.MIMEType),
			FileSize:        input.FileSize,
		}
		if err := tx.Create(&fileRow).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return domainnode.Node{}, err
	}

	return r.FindViewByID(ctx, createdID, input.LibraryID)
}

// UpdateNodeFields 更新节点元数据字段。
func (r *NodeRepository) UpdateNodeFields(ctx context.Context, nodeID, libraryID uint64, updates map[string]any) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&nodeModel{}).
		Where("id = ? AND library_id = ?", nodeID, libraryID).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// RenameNode 在同级目录内重命名节点。
func (r *NodeRepository) RenameNode(ctx context.Context, nodeID, libraryID uint64, name string, updatedAt time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := r.WithTx(tx).findNodeModel(ctx, nodeID, libraryID)
		if err != nil {
			return err
		}

		duplicate, err := r.WithTx(tx).hasDuplicateName(ctx, name, parentIDValue(current.ParentID), libraryID, nodeID)
		if err != nil {
			return err
		}
		if duplicate {
			return ErrConflict
		}

		updated, err := r.WithTx(tx).UpdateNodeFields(ctx, nodeID, libraryID, map[string]any{
			"name":       name,
			"updated_at": updatedAt,
		})
		if err != nil {
			return err
		}
		if !updated {
			return ErrNotFound
		}
		return nil
	})
}

// MoveNode 处理节点移动与重排，保持同一父级下排序稳定。
func (r *NodeRepository) MoveNode(ctx context.Context, input MoveNodeInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		node, err := r.WithTx(tx).findNodeModel(ctx, input.NodeID, input.LibraryID)
		if err != nil {
			return err
		}

		if input.NewParentID == input.NodeID {
			return fmt.Errorf("%w: node cannot be moved under itself", ErrInvalidState)
		}

		if input.NewParentID > 0 {
			newParent, err := r.WithTx(tx).findNodeModel(ctx, input.NewParentID, input.LibraryID)
			if err != nil {
				return err
			}
			if newParent.NodeType != nodeTypeDirectory {
				return fmt.Errorf("%w: target parent must be directory", ErrInvalidState)
			}
		}

		if input.NewParentID > 0 {
			isDescendant, err := r.WithTx(tx).isDescendant(ctx, input.NodeID, input.NewParentID, input.LibraryID)
			if err != nil {
				return err
			}
			if isDescendant {
				return fmt.Errorf("%w: cannot move node under descendant", ErrInvalidState)
			}
		}

		name := strings.TrimSpace(input.Name)
		if name == "" {
			name = node.Name
		}
		duplicate, err := r.WithTx(tx).hasDuplicateName(ctx, name, input.NewParentID, input.LibraryID, input.NodeID)
		if err != nil {
			return err
		}
		if duplicate {
			return ErrConflict
		}

		newOrder, err := r.WithTx(tx).resolveMoveSortOrder(ctx, input)
		if err != nil {
			return err
		}

		updated, err := r.WithTx(tx).UpdateNodeFields(ctx, input.NodeID, input.LibraryID, map[string]any{
			"parent_id":  normalizeParentID(input.NewParentID),
			"sort_order": newOrder,
			"updated_at": input.UpdatedAt,
		})
		if err != nil {
			return err
		}
		if !updated {
			return ErrNotFound
		}
		return nil
	})
}

func (r *NodeRepository) resolveMoveSortOrder(ctx context.Context, input MoveNodeInput) (int, error) {
	if input.BeforeNodeID > 0 {
		beforeNode, err := r.findNodeModel(ctx, input.BeforeNodeID, input.LibraryID)
		if err != nil {
			return 0, err
		}
		if parentIDValue(beforeNode.ParentID) != input.NewParentID {
			return 0, fmt.Errorf("%w: before node is not under target parent", ErrInvalidState)
		}

		shift := r.db.WithContext(ctx).
			Model(&nodeModel{}).
			Where("library_id = ? AND sort_order >= ?", input.LibraryID, beforeNode.SortOrder).
			Where("deleted_at IS NULL")
		shift = applyParentFilter(shift, input.NewParentID)
		if err := shift.Update("sort_order", gorm.Expr("sort_order + 1")).Error; err != nil {
			return 0, err
		}
		return beforeNode.SortOrder, nil
	}

	maxSort, err := r.maxSortOrder(ctx, input.LibraryID, input.NewParentID)
	if err != nil {
		return 0, err
	}
	return maxSort + 15, nil
}

func (r *NodeRepository) findNodeModel(ctx context.Context, nodeID, libraryID uint64) (nodeModel, error) {
	var row nodeModel
	if err := r.db.WithContext(ctx).First(&row, "id = ? AND library_id = ?", nodeID, libraryID).Error; err != nil {
		return nodeModel{}, mapDBError(err)
	}
	return row, nil
}

func (r *NodeRepository) hasDuplicateName(ctx context.Context, name string, parentID, libraryID, excludeID uint64) (bool, error) {
	query := r.db.WithContext(ctx).Model(&nodeModel{}).
		Where("name = ? AND library_id = ?", name, libraryID).
		Where("deleted_at IS NULL")
	query = applyParentFilter(query, parentID)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *NodeRepository) maxSortOrder(ctx context.Context, libraryID, parentID uint64) (int, error) {
	query := r.db.WithContext(ctx).Model(&nodeModel{}).
		Select("COALESCE(MAX(sort_order), 0)").
		Where("library_id = ?", libraryID).
		Where("deleted_at IS NULL")
	query = applyParentFilter(query, parentID)

	var maxSort int
	if err := query.Scan(&maxSort).Error; err != nil {
		return 0, err
	}
	return maxSort, nil
}

func (r *NodeRepository) isDescendant(ctx context.Context, nodeID, targetID, libraryID uint64) (bool, error) {
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

	var count int64
	if err := r.db.WithContext(ctx).Raw(query, nodeID, libraryID, libraryID, targetID).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func nodeTypeCode(t domainnode.Type) int {
	if t == domainnode.TypeFile {
		return nodeTypeFile
	}
	return nodeTypeDirectory
}

func normalizeParentID(parentID uint64) *uint64 {
	if parentID == 0 {
		return nil
	}
	value := parentID
	return &value
}

func applyParentFilter(query *gorm.DB, parentID uint64) *gorm.DB {
	if parentID == 0 {
		return query.Where("parent_id IS NULL")
	}
	return query.Where("parent_id = ?", parentID)
}
