package repository

import (
	domain "omniflow-go/internal/domain/resourcemonitor"
	resourcemonitorpg "omniflow-go/internal/repository/postgres/impl/resourcemonitor"

	"gorm.io/gorm"
)

// NewResourceMonitorRepository 创建资源监测只读仓储。
func NewResourceMonitorRepository(db *gorm.DB) domain.Repository {
	return resourcemonitorpg.NewRepository(db)
}
