package repository

import (
	"context"
	"sort"

	domainnode "omniflow-go/internal/domain/node"
)

// ListAllDescendants 查询节点整棵子树，并按深度与排序返回。
func (r *NodeRepository) ListAllDescendants(ctx context.Context, nodeID, libraryID uint64) ([]domainnode.Node, error) {
	var refs []descendantRow
	if err := r.scanRaw(ctx, &refs, sqlListTreeDescendantRefs, nodeID, libraryID, libraryID); err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return []domainnode.Node{}, nil
	}

	ids := make([]uint64, 0, len(refs))
	for _, item := range refs {
		ids = append(ids, item.ID)
	}

	depthByID := make(map[uint64]int, len(refs))
	for _, item := range refs {
		depthByID[item.ID] = item.Depth
	}

	nodes, err := r.loadNodesWithFileMeta(ctx, libraryID, ids, depthByID)
	if err != nil {
		return nil, err
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Depth != nodes[j].Depth {
			return nodes[i].Depth < nodes[j].Depth
		}
		if nodes[i].SortOrder != nodes[j].SortOrder {
			return nodes[i].SortOrder < nodes[j].SortOrder
		}
		return nodes[i].Node.ID < nodes[j].Node.ID
	})

	result := make([]domainnode.Node, 0, len(nodes))
	for _, item := range nodes {
		result = append(result, item.Node)
	}
	return result, nil
}

// ListDirectChildren 查询单层子节点，保持同级排序。
func (r *NodeRepository) ListDirectChildren(ctx context.Context, nodeID, libraryID uint64) ([]domainnode.Node, error) {
	var rows []nodesEntity
	if err := r.dbWithContext(ctx).
		Where("library_id = ? AND parent_id = ?", libraryID, nodeID).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []domainnode.Node{}, nil
	}

	ids := make([]uint64, 0, len(rows))
	order := make(map[uint64]int, len(rows))
	for i, row := range rows {
		ids = append(ids, row.ID)
		order[row.ID] = i
	}

	nodes, err := r.loadNodesWithFileMeta(ctx, libraryID, ids, nil)
	if err != nil {
		return nil, err
	}

	sort.Slice(nodes, func(i, j int) bool {
		return order[nodes[i].Node.ID] < order[nodes[j].Node.ID]
	})

	result := make([]domainnode.Node, 0, len(nodes))
	for _, item := range nodes {
		result = append(result, item.Node)
	}
	return result, nil
}

// ListAncestors 查询从当前节点向上的祖先链。
func (r *NodeRepository) ListAncestors(ctx context.Context, nodeID, libraryID uint64) ([]NodePathItem, error) {
	var rows []NodePathItem
	if err := r.scanRaw(ctx, &rows, sqlListAncestorPath, nodeID, libraryID, libraryID); err != nil {
		return nil, err
	}
	return rows, nil
}

// FindViewByID 查询单个节点并补齐文件元信息。
func (r *NodeRepository) FindViewByID(ctx context.Context, nodeID, libraryID uint64) (domainnode.Node, error) {
	var row nodesEntity
	if err := r.dbWithContext(ctx).
		Where("id = ? AND library_id = ?", nodeID, libraryID).
		First(&row).Error; err != nil {
		return domainnode.Node{}, mapDBError(err)
	}

	nodes, err := r.loadNodesWithFileMeta(ctx, libraryID, []uint64{row.ID}, nil)
	if err != nil {
		return domainnode.Node{}, err
	}
	if len(nodes) == 0 {
		return domainnode.Node{}, ErrNotFound
	}
	return nodes[0].Node, nil
}

// loadNodesWithFileMeta 通过分表查询补齐 MIME/FileSize/StorageKey，避免读路径 join。
func (r *NodeRepository) loadNodesWithFileMeta(ctx context.Context, libraryID uint64, ids []uint64, depthByID map[uint64]int) ([]nodeWithSort, error) {
	if len(ids) == 0 {
		return []nodeWithSort{}, nil
	}

	var rows []nodesEntity
	if err := r.dbWithContext(ctx).
		Where("library_id = ? AND id IN ?", libraryID, ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []nodeWithSort{}, nil
	}

	fileIDs := make([]uint64, 0, len(rows))
	assembled := make(map[uint64]nodeWithSort, len(rows))
	for _, row := range rows {
		depth := 0
		if depthByID != nil {
			depth = depthByID[row.ID]
		}

		node := row.toDomainNode()
		assembled[row.ID] = nodeWithSort{
			Node:      node,
			Depth:     depth,
			SortOrder: row.SortOrder,
		}
		if row.NodeType == nodeTypeFile {
			fileIDs = append(fileIDs, row.ID)
		}
	}

	if len(fileIDs) == 0 {
		output := make([]nodeWithSort, 0, len(assembled))
		for _, item := range assembled {
			output = append(output, item)
		}
		return output, nil
	}

	var fileRows []nodeFilesEntity
	if err := r.dbWithContext(ctx).
		Where("library_id = ? AND file_id IN ?", libraryID, fileIDs).
		Find(&fileRows).Error; err != nil {
		return nil, err
	}

	storageIDs := make([]uint64, 0, len(fileRows))
	fileByID := make(map[uint64]nodeFilesEntity, len(fileRows))
	for _, row := range fileRows {
		fileByID[row.FileID] = row
		if row.StorageObjectID > 0 {
			storageIDs = append(storageIDs, row.StorageObjectID)
		}
	}

	storageByID := map[uint64]storageObjectsEntity{}
	if len(storageIDs) > 0 {
		var storageRows []storageObjectsEntity
		if err := r.dbWithContext(ctx).
			Where("library_id = ? AND id IN ?", libraryID, storageIDs).
			Find(&storageRows).Error; err != nil {
			return nil, err
		}
		for _, row := range storageRows {
			storageByID[row.ID] = row
		}
	}

	for nodeID, item := range assembled {
		fileRow, ok := fileByID[nodeID]
		if !ok {
			continue
		}

		item.Node.MIMEType = fileRow.MIMEType
		item.Node.FileSize = fileRow.FileSize
		if storage, ok := storageByID[fileRow.StorageObjectID]; ok {
			item.Node.StorageKey = storage.ObjectKey
		}
		assembled[nodeID] = item
	}

	output := make([]nodeWithSort, 0, len(assembled))
	for _, item := range assembled {
		output = append(output, item)
	}
	return output, nil
}
