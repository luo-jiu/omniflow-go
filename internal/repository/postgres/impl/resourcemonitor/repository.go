package repository

import (
	"context"
	"errors"

	domain "omniflow-go/internal/domain/resourcemonitor"

	"gorm.io/gorm"
)

// Repository 提供资源监测相关的 PostgreSQL 只读查询。
type Repository struct {
	db *gorm.DB
}

// NewRepository 创建资源监测仓储。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) dbWithContext(ctx context.Context) *gorm.DB {
	if r.db == nil {
		return nil
	}
	return r.db.WithContext(ctx)
}

// Ping 执行 PostgreSQL 只读连通性检查。
func (r *Repository) Ping(ctx context.Context) error {
	if r.db == nil {
		return errors.New("resource monitor repository: database is nil")
	}
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// CountStorageDistribution 统计指定用户可见资料库范围内的物理存储分布。
func (r *Repository) CountStorageDistribution(
	ctx context.Context,
	ownerUserID uint64,
) ([]domain.StorageDistributionRow, error) {
	if r.db == nil {
		return nil, errors.New("resource monitor repository: database is nil")
	}

	const querySQL = `
		WITH scoped_objects AS (
			SELECT
				so.id,
				so.provider,
				so.bucket,
				so.content_length
			FROM storage_objects so
			JOIN libraries l
			  ON l.id = so.library_id
			 AND l.deleted_at IS NULL
			WHERE so.deleted_at IS NULL
			  AND l.user_id = ?
		),
		file_refs AS (
			SELECT
				nf.storage_object_id,
				COUNT(*) AS file_ref_count
			FROM node_files nf
			JOIN scoped_objects so
			  ON so.id = nf.storage_object_id
			GROUP BY nf.storage_object_id
		)
		SELECT
			so.provider AS provider,
			so.bucket AS bucket,
			COUNT(*) AS object_count,
			COALESCE(SUM(fr.file_ref_count), 0) AS file_ref_count,
			COALESCE(SUM(so.content_length), 0) AS physical_bytes
		FROM scoped_objects so
		LEFT JOIN file_refs fr
		  ON fr.storage_object_id = so.id
		GROUP BY so.provider, so.bucket
		ORDER BY physical_bytes DESC, object_count DESC, provider ASC, bucket ASC
	`

	var rows []domain.StorageDistributionRow
	if err := r.dbWithContext(ctx).
		Raw(querySQL, int64(ownerUserID)).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
