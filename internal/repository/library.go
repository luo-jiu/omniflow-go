package repository

import "gorm.io/gorm"

type LibraryRepository struct {
	db *gorm.DB
}

func NewLibraryRepository(db *gorm.DB) *LibraryRepository {
	return &LibraryRepository{db: db}
}

func (r *LibraryRepository) WithTx(tx *gorm.DB) *LibraryRepository {
	if tx == nil {
		return r
	}
	return &LibraryRepository{db: tx}
}
