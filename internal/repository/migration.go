package repository

import (
	migrationpg "omniflow-go/internal/repository/postgres/impl/migration"

	"gorm.io/gorm"
)

// MigrationRepository 是迁移任务仓储类型别名，便于 wire 注入。
type MigrationRepository = migrationpg.MigrationRepository

// NewMigrationRepository 构造迁移仓储。
func NewMigrationRepository(db *gorm.DB) *MigrationRepository {
	return migrationpg.NewMigrationRepository(db)
}
