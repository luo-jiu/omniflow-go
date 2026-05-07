package repository

import (
	domain "omniflow-go/internal/domain/uploadsession"
	pgmodel "omniflow-go/internal/repository/postgres/model"
)

func toDomain(row *pgmodel.UploadSession) domain.UploadSession {
	if row == nil {
		return domain.UploadSession{}
	}
	var parentID uint64
	if row.ParentID != nil && *row.ParentID > 0 {
		parentID = uint64(*row.ParentID)
	}
	var minioUploadID string
	if row.MinioUploadID != nil {
		minioUploadID = *row.MinioUploadID
	}
	return domain.UploadSession{
		ID:              row.ID,
		LibraryID:       uint64(row.LibraryID),
		ParentID:        parentID,
		ActorID:         row.ActorID,
		StorageKey:      row.StorageKey,
		FileName:        row.FileName,
		FileSize:        row.FileSize,
		ContentType:     row.ContentType,
		StorageProvider: row.StorageProvider,
		Mode:            row.Mode,
		MinioUploadID:   minioUploadID,
		PartSize:        row.PartSize,
		ExpiresAt:       row.ExpiresAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func nullableInt64(value int64) *int64 {
	if value == 0 {
		return nil
	}
	v := value
	return &v
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}
