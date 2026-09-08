package repository

import (
	"context"
	"gorm.io/gorm"
	domainnode "omniflow-go/internal/domain/node"
	pgmodel "omniflow-go/internal/repository/postgres/model"
	"strings"
	"time"
)

// ReadMetadataRoot 只读取既有根节点，不触发根目录创建或父引用修复。
func (r *NodeRepository) ReadMetadataRoot(ctx context.Context, libraryID uint64) (domainnode.MetadataEntry, error) {
	root, err := r.findLibraryRootNode(ctx, libraryID)
	if err != nil {
		return domainnode.MetadataEntry{}, mapDBError(err)
	}
	entry := metadataEntry(toDomainNodeModel(root), "/")
	return entry, nil
}

// ReadMetadataNode 读取当前资料库中未删除的节点。
func (r *NodeRepository) ReadMetadataNode(ctx context.Context, nodeID, libraryID uint64) (domainnode.MetadataEntry, error) {
	node, err := r.FindViewByID(ctx, nodeID, libraryID)
	if err != nil {
		return domainnode.MetadataEntry{}, err
	}
	return metadataEntry(node, ""), nil
}

func metadataEntry(node domainnode.Node, path string) domainnode.MetadataEntry {
	if node.Type == domainnode.TypeFile && node.Ext != "" && path != "" &&
		!strings.HasSuffix(strings.ToLower(node.Name), "."+strings.ToLower(node.Ext)) {
		path += "." + node.Ext
	}
	entry := domainnode.MetadataEntry{ID: node.ID, LibraryID: node.LibraryID, ParentID: node.ParentID,
		Name: node.Name, Type: node.Type, Path: path, Ext: node.Ext, MIMEType: node.MIMEType,
		FileSize: node.FileSize, StorageProvider: node.StorageProvider, StorageProviderLabel: node.StorageProviderLabel}
	if !node.UpdatedAt.IsZero() {
		entry.UpdatedAt = node.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	return entry
}

func metadataQuery(db *gorm.DB, input domainnode.MetadataQuery, rootID, afterID uint64, limit int) *gorm.DB {
	q := db.Model(&pgmodel.Node{}).Where("library_id = ? AND id > ? AND id <> ?", input.LibraryID, afterID, rootID)
	if input.NodeID > 0 {
		q = q.Where("id = ?", input.NodeID)
	}
	if input.Mode == "children" {
		q = q.Where("parent_id = ?", input.ParentID)
	}
	if input.NodeType != "" {
		nodeType := nodeTypeDirectory
		if input.NodeType == domainnode.TypeFile {
			nodeType = nodeTypeFile
		}
		q = q.Where("node_type = ?", nodeType)
	}
	if len(input.Names) > 0 {
		q = q.Where("name IN ?", input.Names)
	}
	if input.Keyword != "" {
		keyword := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(input.Keyword)
		q = q.Where("name ILIKE ?", "%"+keyword+"%")
	}
	if input.AncestorID > 0 {
		q = q.Where("id IN ("+sqlMetadataSubtreeIDs+")", input.AncestorID, input.LibraryID, input.LibraryID)
	}
	if len(input.TagIDs) > 0 {
		tags := db.Model(&pgmodel.NodeTagRel{}).Select("node_id").Where("library_id = ? AND tag_id IN ?", input.LibraryID, input.TagIDs).Group("node_id")
		if input.TagMatchMode == "ALL" {
			tags = tags.Having("COUNT(DISTINCT tag_id) = ?", len(input.TagIDs))
		}
		q = q.Where("id IN (?)", tags)
	}
	return q.Order("id ASC").Limit(limit)
}

// QueryMetadata 在数据库中完成分页，只为当前页补齐文件元数据和祖先路径。
func (r *NodeRepository) QueryMetadata(ctx context.Context, input domainnode.MetadataQuery, rootID, afterID uint64, limit int) ([]domainnode.MetadataEntry, error) {
	var rows []*pgmodel.Node
	if err := metadataQuery(r.dbWithContext(ctx), input, rootID, afterID, limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domainnode.MetadataEntry, 0, len(rows))
	if len(rows) == 0 {
		return result, nil
	}
	ids := make([]uint64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, toDomainUint64(row.ID))
	}
	nodes, err := r.loadNodesWithFileMeta(ctx, input.LibraryID, ids, nil)
	if err != nil {
		return nil, err
	}
	var paths []struct {
		ID     uint64
		Path   string
		Rooted bool
	}
	if err := r.scanRaw(ctx, &paths, sqlMetadataPaths, input.LibraryID, toPGInt64Slice(ids), input.LibraryID, rootID, rootID); err != nil {
		return nil, err
	}
	pathByID := make(map[uint64]string, len(paths))
	for _, item := range paths {
		if item.Rooted {
			pathByID[item.ID] = item.Path
		}
	}
	byID := make(map[uint64]domainnode.Node, len(nodes))
	for _, row := range rows {
		byID[toDomainUint64(row.ID)] = toDomainNodeModel(row)
	}
	for _, item := range nodes {
		byID[item.Node.ID] = item.Node
	}
	for _, id := range ids {
		if node, ok := byID[id]; ok {
			result = append(result, metadataEntry(node, pathByID[id]))
		}
	}
	return result, nil
}
