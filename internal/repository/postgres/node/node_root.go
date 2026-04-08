package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

const defaultRootNodeName = "ROOT"

// EnsureLibraryRootNodeID 返回资料库根节点 ID；若不存在则自动创建。
func (r *NodeRepository) EnsureLibraryRootNodeID(ctx context.Context, libraryID uint64) (uint64, error) {
	var root nodesEntity
	err := r.dbWithContext(ctx).
		Where("library_id = ? AND parent_id IS NULL", libraryID).
		Order("id ASC").
		First(&root).Error
	if err == nil {
		return root.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	root = nodesEntity{
		Name:      defaultRootNodeName,
		BuiltIn:   "DEF",
		NodeType:  nodeTypeDirectory,
		Archive:   false,
		SortOrder: 15,
		ParentID:  nil,
		LibraryID: libraryID,
	}
	if err := r.dbWithContext(ctx).Create(&root).Error; err != nil {
		return 0, err
	}
	return root.ID, nil
}
