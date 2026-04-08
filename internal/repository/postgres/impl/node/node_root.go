package repository

import (
	"context"
	"errors"

	pgmodel "omniflow-go/internal/repository/postgres/model"

	"gorm.io/gorm"
)

const defaultRootNodeName = "ROOT"

// EnsureLibraryRootNodeID 返回资料库根节点 ID；若不存在则自动创建。
func (r *NodeRepository) EnsureLibraryRootNodeID(ctx context.Context, libraryID uint64) (uint64, error) {
	q := r.query(ctx)
	root, err := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ParentID.IsNull(),
		).
		Order(q.Node.ID.Asc()).
		First()
	if err == nil {
		return toDomainUint64(root.ID), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	root = &pgmodel.Node{
		Name:        defaultRootNodeName,
		BuiltInType: "DEF",
		NodeType:    nodeTypeDirectory,
		ArchiveMode: false,
		ViewMeta:    "{}",
		SortOrder:   15,
		ParentID:    nil,
		LibraryID:   toPGInt64(libraryID),
	}
	if err := q.Node.WithContext(ctx).Create(root); err != nil {
		return 0, err
	}
	return toDomainUint64(root.ID), nil
}
