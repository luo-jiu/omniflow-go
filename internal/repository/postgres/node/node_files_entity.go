package repository

// nodeFilesEntity 对应 node_files 表结构。
type nodeFilesEntity struct {
	FileID          uint64 `gorm:"column:file_id;primaryKey"`
	LibraryID       uint64 `gorm:"column:library_id"`
	StorageObjectID uint64 `gorm:"column:storage_object_id"`
	MIMEType        string `gorm:"column:mime_type"`
	FileSize        int64  `gorm:"column:file_size"`
}

func (nodeFilesEntity) TableName() string {
	return "node_files"
}
