package repository

import (
	userpreferencepg "omniflow-go/internal/repository/postgres/impl/userpreference"

	"gorm.io/gorm"
)

type UserPreferenceRepository = userpreferencepg.UserPreferenceRepository
type CreateUserPreferenceInput = userpreferencepg.CreateUserPreferenceInput
type UpdateUserPreferenceInput = userpreferencepg.UpdateUserPreferenceInput

func NewUserPreferenceRepository(db *gorm.DB) *UserPreferenceRepository {
	return userpreferencepg.NewUserPreferenceRepository(db)
}
