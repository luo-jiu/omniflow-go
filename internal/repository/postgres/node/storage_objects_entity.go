package repository

import "gorm.io/gorm"

// storageObjectsEntity 对应 storage_objects 表结构。
type storageObjectsEntity struct {
	ID            uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	LibraryID     uint64         `gorm:"column:library_id"`
	Provider      string         `gorm:"column:provider"`
	Bucket        string         `gorm:"column:bucket"`
	ObjectKey     string         `gorm:"column:object_key"`
	ContentLength int64          `gorm:"column:content_length"`
	ContentType   string         `gorm:"column:content_type"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (storageObjectsEntity) TableName() string {
	return "storage_objects"
}
