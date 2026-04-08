package repository

import (
	userpg "omniflow-go/internal/repository/postgres/impl/user"

	"gorm.io/gorm"
)

type UserRepository = userpg.UserRepository
type UserAuth = userpg.UserAuth
type CreateUserInput = userpg.CreateUserInput

func NewUserRepository(db *gorm.DB) *UserRepository {
	return userpg.NewUserRepository(db)
}
