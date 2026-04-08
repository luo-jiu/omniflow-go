package repository

import (
	librarypg "omniflow-go/internal/repository/postgres/impl/library"

	"gorm.io/gorm"
)

type LibraryRepository = librarypg.LibraryRepository

func NewLibraryRepository(db *gorm.DB) *LibraryRepository {
	return librarypg.NewLibraryRepository(db)
}
