package repository

import (
	"gorm.io/gorm"

	browserbookmarkpg "omniflow-go/internal/repository/postgres/impl/browserbookmark"
)

type BrowserBookmarkRepository = browserbookmarkpg.BrowserBookmarkRepository
type CreateBrowserBookmarkInput = browserbookmarkpg.CreateBrowserBookmarkInput
type UpdateBrowserBookmarkInput = browserbookmarkpg.UpdateBrowserBookmarkInput
type BrowserBookmarkSortOrder = browserbookmarkpg.BrowserBookmarkSortOrder

func NewBrowserBookmarkRepository(db *gorm.DB) *BrowserBookmarkRepository {
	return browserbookmarkpg.NewBrowserBookmarkRepository(db)
}
