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

	q := r.query(ctx)
	_, err = q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ID.In(toPGInt64Slice(descendantIDs)...),
		).
		Delete()
	if err != nil {
		return DeleteNodeTreeResult{}, err
	}

	return DeleteNodeTreeResult{
		DeletedNodeCount: len(descendantIDs),
		FileNodeCount:    fileCount,
	}, nil
}

func (r *NodeRepository) listDescendantIDs(ctx context.Context, nodeID, libraryID uint64) ([]uint64, error) {
	var rawIDs []int64
	if err := r.scanRaw(ctx, &rawIDs, sqlListSubtreeNodeIDs, nodeID, libraryID, libraryID); err != nil {
		return nil, err
	}
	return toDomainUint64Slice(rawIDs), nil
}

func (r *NodeRepository) countFileNodesWithStorageKey(ctx context.Context, libraryID uint64, nodeIDs []uint64) (int64, error) {
	if len(nodeIDs) == 0 {
		return 0, nil
	}

	q := r.query(ctx)
	fileRows, err := q.NodeFile.WithContext(ctx).
		Where(
			q.NodeFile.LibraryID.Eq(toPGInt64(libraryID)),
			q.NodeFile.FileID.In(toPGInt64Slice(nodeIDs)...),
		).
		Find()
	if err != nil {
		return 0, err
	}
	if len(fileRows) == 0 {
		return 0, nil
	}

	storageIDs := make([]int64, 0, len(fileRows))
	for _, row := range fileRows {
		if row.StorageObjectID > 0 {
			storageIDs = append(storageIDs, row.StorageObjectID)
		}
	}
	if len(storageIDs) == 0 {
		return 0, nil
	}

	storageRows, err := q.StorageObject.WithContext(ctx).
		Select(q.StorageObject.ID, q.StorageObject.ObjectKey).
		Where(
			q.StorageObject.LibraryID.Eq(toPGInt64(libraryID)),
			q.StorageObject.ID.In(storageIDs...),
			q.StorageObject.ObjectKey.Neq(""),
		).
		Find()
	if err != nil {
		return 0, err
	}

	aliveStorage := make(map[uint64]struct{}, len(storageRows))
	for _, row := range storageRows {
		aliveStorage[toDomainUint64(row.ID)] = struct{}{}
	}

	var count int64
	for _, row := range fileRows {
		if _, ok := aliveStorage[toDomainUint64(row.StorageObjectID)]; ok {
			count++
		}
	}
	return count, nil
}
