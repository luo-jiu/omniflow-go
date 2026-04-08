package repository

import (
	"context"
	"errors"
	"time"

	domainnode "omniflow-go/internal/domain/node"
	pgmodel "omniflow-go/internal/repository/postgres/model"
)

// ListDeletedNodes 查询资料库内已进入回收站的节点（全量，用于上层组装顶层条目）。
func (r *NodeRepository) ListDeletedNodes(ctx context.Context, libraryID uint64) ([]domainnode.RecycleItem, error) {
	db := r.dbWithContext(ctx).Unscoped()

	var rows []*pgmodel.Node
	if err := db.
		Where("library_id = ? AND deleted_at IS NOT NULL", toPGInt64(libraryID)).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []domainnode.RecycleItem{}, nil
	}

	items := make([]domainnode.RecycleItem, 0, len(rows))
	indexByID := make(map[uint64]int, len(rows))
	fileIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		nodeType := domainnode.TypeDirectory
		if row.NodeType == nodeTypeFile {
			nodeType = domainnode.TypeFile
			fileIDs = append(fileIDs, row.ID)
		}

		item := domainnode.RecycleItem{
			ID:        toDomainUint64(row.ID),
			Name:      row.Name,
			Type:      nodeType,
			ParentID:  parentIDValue(row.ParentID),
			LibraryID: toDomainUint64(row.LibraryID),
			Ext:       derefString(row.Ext),
		}
		if row.DeletedAt.Valid {
			item.DeletedAt = row.DeletedAt.Time
		}

		indexByID[item.ID] = len(items)
		items = append(items, item)
	}

	if len(fileIDs) == 0 {
		return items, nil
	}

	q := r.query(ctx)
	fileRows, err := q.NodeFile.WithContext(ctx).
		Where(
			q.NodeFile.LibraryID.Eq(toPGInt64(libraryID)),
			q.NodeFile.FileID.In(fileIDs...),
		).
		Find()
	if err != nil {
		return nil, err
	}

	for _, row := range fileRows {
		nodeID := toDomainUint64(row.FileID)
		idx, ok := indexByID[nodeID]
		if !ok {
			continue
		}
		items[idx].MIMEType = derefString(row.MimeType)
		items[idx].FileSize = row.FileSize
	}
	return items, nil
}

// RestoreTree 恢复回收站中的节点子树。
func (r *NodeRepository) RestoreTree(ctx context.Context, nodeID, libraryID uint64) (bool, error) {
	target, err := r.findNodeModelIncludingDeleted(ctx, nodeID, libraryID)
	if err != nil {
		return false, err
	}
	if !target.DeletedAt.Valid {
		return true, nil
	}

	descendantIDs, err := r.listDescendantIDsAny(ctx, nodeID, libraryID)
	if err != nil {
		return false, err
	}
	if len(descendantIDs) == 0 {
		return false, ErrNotFound
	}

	rows, err := r.listNodesIncludingDeletedByIDs(ctx, libraryID, descendantIDs)
	if err != nil {
		return false, err
	}
	restoreSet := make(map[uint64]struct{}, len(rows))
	for _, row := range rows {
		restoreSet[toDomainUint64(row.ID)] = struct{}{}
	}

	for _, row := range rows {
		if !row.DeletedAt.Valid {
			continue
		}

		parentID := parentIDValue(row.ParentID)
		if parentID > 0 {
			if _, ok := restoreSet[parentID]; !ok {
				parent, parentErr := r.findNodeModelIncludingDeleted(ctx, parentID, libraryID)
				if parentErr != nil {
					if errors.Is(parentErr, ErrNotFound) {
						return false, ErrInvalidState
					}
					return false, parentErr
				}
				if parent.DeletedAt.Valid {
					return false, ErrInvalidState
				}
			}
		}

		if _, ok := restoreSet[parentID]; ok {
			continue
		}

		duplicate, dupErr := r.hasDuplicateName(
			ctx,
			row.Name,
			parentID,
			libraryID,
			toDomainUint64(row.ID),
		)
		if dupErr != nil {
			return false, dupErr
		}
		if duplicate {
			return false, ErrConflict
		}
	}

	now := time.Now().UTC()
	if err := r.dbWithContext(ctx).Unscoped().
		Model(&pgmodel.Node{}).
		Where(
			"library_id = ? AND id IN ? AND deleted_at IS NOT NULL",
			toPGInt64(libraryID),
			toPGInt64Slice(descendantIDs),
		).
		Updates(map[string]any{
			"deleted_at": nil,
			"updated_at": now,
		}).Error; err != nil {
		return false, err
	}
	return true, nil
}

// HardDeleteTree 彻底删除回收站中的节点子树。
func (r *NodeRepository) HardDeleteTree(ctx context.Context, nodeID, libraryID uint64) (bool, error) {
	target, err := r.findNodeModelIncludingDeleted(ctx, nodeID, libraryID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return true, nil
		}
		return false, err
	}
	if !target.DeletedAt.Valid {
		return false, ErrInvalidState
	}

	descendantIDs, err := r.listDescendantIDsAny(ctx, nodeID, libraryID)
	if err != nil {
		return false, err
	}
	if len(descendantIDs) == 0 {
		return true, nil
	}

	if err := r.dbWithContext(ctx).Unscoped().
		Where(
			"library_id = ? AND id IN ? AND deleted_at IS NOT NULL",
			toPGInt64(libraryID),
			toPGInt64Slice(descendantIDs),
		).
		Delete(&pgmodel.Node{}).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *NodeRepository) listDescendantIDsAny(ctx context.Context, nodeID, libraryID uint64) ([]uint64, error) {
	var rawIDs []int64
	if err := r.scanRaw(ctx, &rawIDs, sqlListSubtreeNodeIDsAny, nodeID, libraryID, libraryID); err != nil {
		return nil, err
	}
	return toDomainUint64Slice(rawIDs), nil
}

func (r *NodeRepository) listNodesIncludingDeletedByIDs(ctx context.Context, libraryID uint64, nodeIDs []uint64) ([]*pgmodel.Node, error) {
	if len(nodeIDs) == 0 {
		return []*pgmodel.Node{}, nil
	}

	db := r.dbWithContext(ctx).Unscoped()
	var rows []*pgmodel.Node
	if err := db.
		Where(
			"library_id = ? AND id IN ?",
			toPGInt64(libraryID),
			toPGInt64Slice(nodeIDs),
		).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
