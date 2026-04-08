package repository

import (
	"context"
	"sort"

	domainnode "omniflow-go/internal/domain/node"
	pgmodel "omniflow-go/internal/repository/postgres/model"
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
	q := r.query(ctx)

	rows, err := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ParentID.Eq(toPGInt64(nodeID)),
		).
		Order(
			q.Node.SortOrder.Asc(),
			q.Node.ID.Asc(),
		).
		Find()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []domainnode.Node{}, nil
	}

	ids := make([]uint64, 0, len(rows))
	order := make(map[uint64]int, len(rows))
	for i, row := range rows {
		id := toDomainUint64(row.ID)
		ids = append(ids, id)
		order[id] = i
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
	q := r.query(ctx)
	row, err := q.Node.WithContext(ctx).
		Where(
			q.Node.ID.Eq(toPGInt64(nodeID)),
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
		).
		First()
	if err != nil {
		return domainnode.Node{}, mapDBError(err)
	}

	nodes, err := r.loadNodesWithFileMeta(ctx, libraryID, []uint64{toDomainUint64(row.ID)}, nil)
	if err != nil {
		return domainnode.Node{}, err
	}
	if len(nodes) == 0 {
		return domainnode.Node{}, ErrNotFound
	}
	return nodes[0].Node, nil
}

// FindViewByNodeID 仅按节点 ID 查询节点视图。
func (r *NodeRepository) FindViewByNodeID(ctx context.Context, nodeID uint64) (domainnode.Node, error) {
	row, err := r.findNodeModelByID(ctx, nodeID)
	if err != nil {
		return domainnode.Node{}, err
	}

	libraryID := toDomainUint64(row.LibraryID)
	nodes, err := r.loadNodesWithFileMeta(ctx, libraryID, []uint64{toDomainUint64(row.ID)}, nil)
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

	q := r.query(ctx)
	rows, err := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ID.In(toPGInt64Slice(ids)...),
		).
		Find()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []nodeWithSort{}, nil
	}

	fileIDs := make([]int64, 0, len(rows))
	assembled := make(map[uint64]nodeWithSort, len(rows))
	for _, row := range rows {
		nodeID := toDomainUint64(row.ID)
		depth := 0
		if depthByID != nil {
			depth = depthByID[nodeID]
		}

		node := toDomainNodeModel(row)
		assembled[nodeID] = nodeWithSort{
			Node:      node,
			Depth:     depth,
			SortOrder: int(row.SortOrder),
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

	fileRows, err := q.NodeFile.WithContext(ctx).
		Where(
			q.NodeFile.LibraryID.Eq(toPGInt64(libraryID)),
			q.NodeFile.FileID.In(fileIDs...),
		).
		Find()
	if err != nil {
		return nil, err
	}

	storageIDs := make([]int64, 0, len(fileRows))
	fileByID := make(map[uint64]*pgmodel.NodeFile, len(fileRows))
	for _, row := range fileRows {
		fileByID[toDomainUint64(row.FileID)] = row
		if row.StorageObjectID > 0 {
			storageIDs = append(storageIDs, row.StorageObjectID)
		}
	}

	storageByID := map[uint64]*pgmodel.StorageObject{}
	if len(storageIDs) > 0 {
		storageRows, err := q.StorageObject.WithContext(ctx).
			Where(
				q.StorageObject.LibraryID.Eq(toPGInt64(libraryID)),
				q.StorageObject.ID.In(storageIDs...),
			).
			Find()
		if err != nil {
			return nil, err
		}
		for _, row := range storageRows {
			storageByID[toDomainUint64(row.ID)] = row
		}
	}

	for nodeID, item := range assembled {
		fileRow, ok := fileByID[nodeID]
		if !ok {
			continue
		}

		item.Node.MIMEType = derefString(fileRow.MimeType)
		item.Node.FileSize = fileRow.FileSize
		if storage, ok := storageByID[toDomainUint64(fileRow.StorageObjectID)]; ok {
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
