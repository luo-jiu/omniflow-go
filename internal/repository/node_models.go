package repository

import (
	domainnode "omniflow-go/internal/domain/node"

	"gorm.io/gorm"
)

type nodeModel struct {
	ID        uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	Name      string         `gorm:"column:name"`
	Ext       *string        `gorm:"column:ext"`
	BuiltIn   string         `gorm:"column:built_in_type"`
	NodeType  int            `gorm:"column:node_type"`
	Archive   bool           `gorm:"column:archive_mode"`
	SortOrder int            `gorm:"column:sort_order"`
	ParentID  *uint64        `gorm:"column:parent_id"`
	LibraryID uint64         `gorm:"column:library_id"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (nodeModel) TableName() string {
	return "nodes"
}

type nodeFileModel struct {
	FileID          uint64 `gorm:"column:file_id;primaryKey"`
	LibraryID       uint64 `gorm:"column:library_id"`
	StorageObjectID uint64 `gorm:"column:storage_object_id"`
	MIMEType        string `gorm:"column:mime_type"`
	FileSize        int64  `gorm:"column:file_size"`
}

func (nodeFileModel) TableName() string {
	return "node_files"
}

type storageObjectModel struct {
	ID            uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	LibraryID     uint64         `gorm:"column:library_id"`
	Provider      string         `gorm:"column:provider"`
	Bucket        string         `gorm:"column:bucket"`
	ObjectKey     string         `gorm:"column:object_key"`
	ContentLength int64          `gorm:"column:content_length"`
	ContentType   string         `gorm:"column:content_type"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (storageObjectModel) TableName() string {
	return "storage_objects"
}

type descendantRow struct {
	ID    uint64 `gorm:"column:id"`
	Depth int    `gorm:"column:depth"`
}

type nodeWithSort struct {
	Node      domainnode.Node
	Depth     int
	SortOrder int
}

func (m nodeModel) toDomain() domainnode.Node {
	ext := ""
	if m.Ext != nil {
		ext = *m.Ext
	}

	nodeType := domainnode.TypeDirectory
	if m.NodeType == nodeTypeFile {
		nodeType = domainnode.TypeFile
	}

	return domainnode.Node{
		ID:          m.ID,
		Name:        m.Name,
		Type:        nodeType,
		ParentID:    parentIDValue(m.ParentID),
		LibraryID:   m.LibraryID,
		Ext:         ext,
		BuiltInType: m.BuiltIn,
		ArchiveMode: boolToArchiveMode(m.Archive),
	}
}

func boolToArchiveMode(enabled bool) int {
	if enabled {
		return 1
	}
	return 0
}

func parentIDValue(parentID *uint64) uint64 {
	if parentID == nil {
		return 0
	}
	return *parentID
}
