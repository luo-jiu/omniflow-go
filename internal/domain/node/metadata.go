package node

import "context"

// MetadataQuery 描述只读节点查询，不依赖对象存储。
type MetadataQuery struct {
	LibraryID    uint64   `json:"libraryId"`
	Mode         string   `json:"mode"`
	ParentID     uint64   `json:"parentId,omitempty"`
	NodeID       uint64   `json:"nodeId,omitempty"`
	AncestorID   uint64   `json:"ancestorId,omitempty"`
	Keyword      string   `json:"keyword,omitempty"`
	Names        []string `json:"names,omitempty"`
	NodeType     Type     `json:"nodeType,omitempty"`
	TagIDs       []uint64 `json:"tagIds,omitempty"`
	TagMatchMode string   `json:"tagMatchMode,omitempty"`
}

// MetadataEntry 仅暴露逻辑路径和安全元数据，不包含对象 Key 或访问地址。
type MetadataEntry struct {
	ID                   uint64 `json:"id"`
	LibraryID            uint64 `json:"libraryId"`
	ParentID             uint64 `json:"parentId"`
	Name                 string `json:"name"`
	Type                 Type   `json:"type"`
	Path                 string `json:"path,omitempty"`
	Ext                  string `json:"ext,omitempty"`
	MIMEType             string `json:"mimeType,omitempty"`
	FileSize             int64  `json:"fileSize"`
	StorageProvider      string `json:"storageProvider,omitempty"`
	StorageProviderLabel string `json:"storageProviderLabel,omitempty"`
	UpdatedAt            string `json:"updatedAt,omitempty"`
}

// MetadataPage 使用与查询条件绑定的游标提供有界结果。
type MetadataPage struct {
	Root       MetadataEntry   `json:"root"`
	Entries    []MetadataEntry `json:"entries"`
	HasMore    bool            `json:"hasMore"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

// MetadataReader 是 Agent、HTTP 与 CLI 共享的只读元数据端口。
type MetadataReader interface {
	ReadMetadataRoot(context.Context, uint64) (MetadataEntry, error)
	ReadMetadataNode(context.Context, uint64, uint64) (MetadataEntry, error)
	QueryMetadata(context.Context, MetadataQuery, uint64, uint64, int) ([]MetadataEntry, error)
}
