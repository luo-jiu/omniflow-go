package repository

import (
	domain "omniflow-go/internal/domain/migration"
	pgmodel "omniflow-go/internal/repository/postgres/model"
)

func taskToDomain(row *pgmodel.MigrationTask) domain.Task {
	if row == nil {
		return domain.Task{}
	}
	return domain.Task{
		ID:               row.ID,
		ActorID:          row.ActorID,
		LibraryID:        row.LibraryID,
		RootNodeID:       row.RootNodeID,
		TargetProvider:   row.TargetProvider,
		Status:           row.Status,
		TotalObjects:     row.TotalObjects,
		CompletedObjects: row.CompletedObjects,
		FailedObjects:    row.FailedObjects,
		SkippedObjects:   row.SkippedObjects,
		TotalBytes:       row.TotalBytes,
		TransferredBytes: row.TransferredBytes,
		CurrentObjectKey: row.CurrentObjectKey,
		ErrorMessage:     row.ErrorMessage,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		StartedAt:        row.StartedAt,
		FinishedAt:       row.FinishedAt,
	}
}

func itemToDomain(row *pgmodel.MigrationTaskItem) domain.TaskItem {
	if row == nil {
		return domain.TaskItem{}
	}
	var targetID int64
	if row.TargetStorageObjectID != nil {
		targetID = *row.TargetStorageObjectID
	}
	return domain.TaskItem{
		ID:                    row.ID,
		TaskID:                row.TaskID,
		StorageObjectID:       row.StorageObjectID,
		SourceProvider:        row.SourceProvider,
		SourceBucket:          row.SourceBucket,
		SourceKey:             row.SourceKey,
		TargetStorageObjectID: targetID,
		TargetKey:             row.TargetKey,
		FileSize:              row.FileSize,
		Status:                row.Status,
		ErrorMessage:          row.ErrorMessage,
		StartedAt:             row.StartedAt,
		FinishedAt:            row.FinishedAt,
		CreatedAt:             row.CreatedAt,
	}
}

func nullableInt64Ptr(value int64) *int64 {
	if value == 0 {
		return nil
	}
	v := value
	return &v
}
