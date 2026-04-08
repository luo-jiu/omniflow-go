package repository

import (
	"context"
)

// DeleteTree 删除节点及其子树，并返回删除统计信息。
func (r *NodeRepository) DeleteTree(ctx context.Context, nodeID, libraryID uint64) (DeleteNodeTreeResult, error) {
	descendantIDs, err := r.listDescendantIDs(ctx, nodeID, libraryID)
	if err != nil {
		return DeleteNodeTreeResult{}, err
	}
	if len(descendantIDs) == 0 {
		return DeleteNodeTreeResult{}, nil
	}

	fileCount, err := r.countFileNodesWithStorageKey(ctx, libraryID, descendantIDs)
	if err != nil {
		return DeleteNodeTreeResult{}, err
	}

	if err := r.dbWithContext(ctx).
		Where("library_id = ? AND id IN ?", libraryID, descendantIDs).
		Delete(&nodesEntity{}).Error; err != nil {
		return DeleteNodeTreeResult{}, err
	}

	return DeleteNodeTreeResult{
		DeletedNodeCount: len(descendantIDs),
		FileNodeCount:    fileCount,
	}, nil
}

func (r *NodeRepository) listDescendantIDs(ctx context.Context, nodeID, libraryID uint64) ([]uint64, error) {
	var ids []uint64
	if err := r.scanRaw(ctx, &ids, sqlListSubtreeNodeIDs, nodeID, libraryID, libraryID); err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *NodeRepository) countFileNodesWithStorageKey(ctx context.Context, libraryID uint64, nodeIDs []uint64) (int64, error) {
	if len(nodeIDs) == 0 {
		return 0, nil
	}

	var fileRows []nodeFilesEntity
	if err := r.dbWithContext(ctx).
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

	var storageRows []storageObjectsEntity
	if err := r.dbWithContext(ctx).
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
