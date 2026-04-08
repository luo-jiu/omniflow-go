package repository

import "gorm.io/gorm"

// nodesEntity 对应 nodes 表结构。
type nodesEntity struct {
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

func (nodesEntity) TableName() string {
	return "nodes"
}
