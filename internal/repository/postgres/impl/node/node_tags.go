package repository

import (
	"context"

	pgmodel "omniflow-go/internal/repository/postgres/model"

	"gorm.io/gorm/clause"
)

// ReplaceNodeTagIDs 用 node_tag_rel 覆盖节点的直接标签关系。
func (r *NodeRepository) ReplaceNodeTagIDs(
	ctx context.Context,
	libraryID, nodeID, ownerUserID uint64,
	tagIDs []uint64,
) error {
	db := r.dbWithContext(ctx)
	normalizedIDs := normalizeRelationTagIDs(tagIDs)
	pgTagIDs := toPGInt64Slice(normalizedIDs)

	if len(pgTagIDs) > 0 {
		if err := r.ensureReadableTags(ctx, ownerUserID, pgTagIDs); err != nil {
			return err
		}
	}

	deleteQuery := db.
		Where("library_id = ? AND node_id = ?", toPGInt64(libraryID), toPGInt64(nodeID))
	if len(pgTagIDs) > 0 {
		deleteQuery = deleteQuery.Where("tag_id NOT IN ?", pgTagIDs)
	}
	if err := deleteQuery.Delete(&pgmodel.NodeTagRel{}).Error; err != nil {
		return err
	}
	if len(pgTagIDs) == 0 {
		return nil
	}

	rows := make([]*pgmodel.NodeTagRel, 0, len(pgTagIDs))
	for _, tagID := range pgTagIDs {
		rows = append(rows, &pgmodel.NodeTagRel{
			NodeID:    toPGInt64(nodeID),
			TagID:     tagID,
			LibraryID: toPGInt64(libraryID),
		})
	}
	if err := db.
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "node_id"}, {Name: "tag_id"}}, DoNothing: true}).
		Create(&rows).Error; err != nil {
		return mapDBError(err)
	}
	return nil
}

func (r *NodeRepository) ensureReadableTags(ctx context.Context, ownerUserID uint64, tagIDs []int64) error {
	var count int64
	if err := r.dbWithContext(ctx).
		Model(&pgmodel.Tag{}).
		Where("id IN ? AND deleted_at IS NULL AND enabled = TRUE", tagIDs).
		Where("(owner_user_id = ? OR owner_user_id IS NULL)", toPGInt64(ownerUserID)).
		Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(tagIDs)) {
		return ErrInvalidState
	}
	return nil
}

func normalizeRelationTagIDs(tagIDs []uint64) []uint64 {
	if len(tagIDs) == 0 {
		return []uint64{}
	}
	seen := make(map[uint64]struct{}, len(tagIDs))
	result := make([]uint64, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		if tagID == 0 {
			continue
		}
		if _, ok := seen[tagID]; ok {
			continue
		}
		seen[tagID] = struct{}{}
		result = append(result, tagID)
	}
	return result
}
