package repository

import (
	tagpg "omniflow-go/internal/repository/postgres/impl/tag"

	"gorm.io/gorm"
)

type TagRepository = tagpg.TagRepository
type CreateTagInput = tagpg.CreateTagInput
type UpdateTagInput = tagpg.UpdateTagInput

func NewTagRepository(db *gorm.DB) *TagRepository {
	return tagpg.NewTagRepository(db)
}
