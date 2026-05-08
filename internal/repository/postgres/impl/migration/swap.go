package repository

import (
	"context"

	pgmodel "omniflow-go/internal/repository/postgres/model"
)

// SwapInput 表示一次 storage_object 物理切换所需的全部参数。
type SwapInput struct {
	OldStorageObjectID int64
	LibraryID          int64
	TargetProvider     string
	TargetBucket       string
	TargetKey          string
	ContentLength      int64
	ContentType        string
	ETag               string
}

// SwapStorageObject 在事务内完成「插入新 storage_object → 切 node_files 引用 → 软删旧 storage_object」三步。
//
// 调用方必须自己用 GormTransactor.WithinTx 包住调用，仓储不私自起事务。
// 返回新插入的 storage_object_id。
func (r *MigrationRepository) SwapStorageObject(
	ctx context.Context,
	in SwapInput,
) (int64, error) {
	db := r.dbWithContext(ctx)

	newRow := &pgmodel.StorageObject{
		LibraryID:     in.LibraryID,
		Provider:      in.TargetProvider,
		Bucket:        in.TargetBucket,
		ObjectKey:     in.TargetKey,
		ContentLength: in.ContentLength,
		Extra:         "{}",
	}
	if in.ContentType != "" {
		ct := in.ContentType
		newRow.ContentType = &ct
	}
	if in.ETag != "" {
		etag := in.ETag
		newRow.Etag = &etag
	}
	if err := db.Create(newRow).Error; err != nil {
		return 0, mapDBError(err)
	}

	// 切引用：所有指向旧 storage_object 的 node_files 一起切到新 id。
	// 即便有多个 node 共享同一物理对象，也保证一次性原子切换。
	if err := db.
		Model(&pgmodel.NodeFile{}).
		Where("library_id = ? AND storage_object_id = ?", in.LibraryID, in.OldStorageObjectID).
		Update("storage_object_id", newRow.ID).Error; err != nil {
		return 0, mapDBError(err)
	}

	// 软删旧 storage_object。GORM 的 DeletedAt 字段会触发 soft delete。
	if err := db.
		Where("id = ?", in.OldStorageObjectID).
		Delete(&pgmodel.StorageObject{}).Error; err != nil {
		return 0, mapDBError(err)
	}

	return newRow.ID, nil
}
