package repository

import (
	"context"

	"gorm.io/gorm"
)

// DeleteTree 删除节点及其子树，并返回删除统计信息。
func (r *NodeRepository) DeleteTree(ctx context.Context, nodeID, libraryID uint64) (DeleteNodeTreeResult, error) {
	result := DeleteNodeTreeResult{}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		descendantIDs, err := r.WithTx(tx).listDescendantIDs(ctx, nodeID, libraryID)
		if err != nil {
			return err
		}
		if len(descendantIDs) == 0 {
			return nil
		}

		fileCount, err := r.WithTx(tx).countFileNodesWithStorageKey(ctx, libraryID, descendantIDs)
		if err != nil {
			return err
		}
		result.FileNodeCount = fileCount
		result.DeletedNodeCount = len(descendantIDs)

		if err := tx.Where("library_id = ? AND id IN ?", libraryID, descendantIDs).Delete(&nodeModel{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return DeleteNodeTreeResult{}, err
	}

	return result, nil
}

func (r *NodeRepository) listDescendantIDs(ctx context.Context, nodeID, libraryID uint64) ([]uint64, error) {
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
SELECT id FROM sub`

	var ids []uint64
	if err := r.db.WithContext(ctx).Raw(query, nodeID, libraryID, libraryID).Scan(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *NodeRepository) countFileNodesWithStorageKey(ctx context.Context, libraryID uint64, nodeIDs []uint64) (int64, error) {
	if len(nodeIDs) == 0 {
		return 0, nil
	}

	var fileRows []nodeFileModel
	if err := r.db.WithContext(ctx).
		Where("library_id = ? AND file_id IN ?", libraryID, nodeIDs).
		Find(&fileRows).Error; err != nil {
		return 0, err
	}
	if len(fileRows) == 0 {
		return 0, nil
	}

	storageIDs := make([]uint64, 0, len(fileRows))
	for _, row := range fileRows {
		if row.StorageObjectID > 0 {
			storageIDs = append(storageIDs, row.StorageObjectID)
		}
	}
	if len(storageIDs) == 0 {
		return 0, nil
	}

	var storageRows []storageObjectModel
	if err := r.db.WithContext(ctx).
		Select("id, object_key").
		Where("library_id = ? AND id IN ? AND object_key <> ''", libraryID, storageIDs).
		Find(&storageRows).Error; err != nil {
		return 0, err
	}

	aliveStorage := make(map[uint64]struct{}, len(storageRows))
	for _, row := range storageRows {
		aliveStorage[row.ID] = struct{}{}
	}

	var count int64
	for _, row := range fileRows {
		if _, ok := aliveStorage[row.StorageObjectID]; ok {
			count++
		}
	}
	return count, nil
}
