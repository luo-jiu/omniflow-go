package repository

import (
	"gorm.io/gorm"

	browserfilemappingpg "omniflow-go/internal/repository/postgres/impl/browserfilemapping"
)

type BrowserFileMappingRepository = browserfilemappingpg.BrowserFileMappingRepository
type CreateBrowserFileMappingInput = browserfilemappingpg.CreateBrowserFileMappingInput
type UpdateBrowserFileMappingInput = browserfilemappingpg.UpdateBrowserFileMappingInput

func NewBrowserFileMappingRepository(db *gorm.DB) *BrowserFileMappingRepository {
	return browserfilemappingpg.NewBrowserFileMappingRepository(db)
}
