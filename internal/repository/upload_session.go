package repository

import (
	uploadsessionpg "omniflow-go/internal/repository/postgres/impl/uploadsession"

	"gorm.io/gorm"
)

type UploadSessionRepository = uploadsessionpg.UploadSessionRepository

func NewUploadSessionRepository(db *gorm.DB) *UploadSessionRepository {
	return uploadsessionpg.NewUploadSessionRepository(db)
}
