package repository

import (
	librarypg "omniflow-go/internal/repository/postgres/library"

	"gorm.io/gorm"
)

type LibraryRepository = librarypg.LibraryRepository

func NewLibraryRepository(db *gorm.DB) *LibraryRepository {
	return librarypg.NewLibraryRepository(db)
}
