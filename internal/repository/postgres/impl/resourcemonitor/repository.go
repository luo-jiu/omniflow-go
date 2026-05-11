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
// libraryID 为 0 时统计用户全部资料库，否则只统计指定资料库。
func (r *Repository) CountStorageDistribution(
	ctx context.Context,
	ownerUserID uint64,
	libraryID uint64,
) ([]domain.StorageDistributionRow, error) {
	if r.db == nil {
		return nil, errors.New("resource monitor repository: database is nil")
	}

	const querySQL = `
		WITH scoped_objects AS (
			SELECT
				so.id,
				so.library_id,
				so.provider,
				so.bucket,
				so.content_length
			FROM storage_objects so
			JOIN libraries l
			  ON l.id = so.library_id
			 AND l.deleted_at IS NULL
			WHERE so.deleted_at IS NULL
			  AND l.user_id = ?
			  AND (? = 0 OR so.library_id = ?)
		),
		object_refs AS (
			SELECT
				so.id AS storage_object_id,
				COUNT(nf.file_id) AS file_ref_count,
				COUNT(nf.file_id) FILTER (
					WHERE n.id IS NOT NULL AND n.deleted_at IS NULL
				) AS visible_file_ref_count,
				COUNT(nf.file_id) FILTER (
					WHERE n.id IS NOT NULL AND n.deleted_at IS NOT NULL
				) AS recycle_file_ref_count,
				BOOL_OR(n.id IS NOT NULL AND n.deleted_at IS NULL) AS has_visible_ref,
				BOOL_OR(n.id IS NOT NULL AND n.deleted_at IS NOT NULL) AS has_recycle_ref
			FROM scoped_objects so
			LEFT JOIN node_files nf
			  ON nf.storage_object_id = so.id
			 AND nf.library_id = so.library_id
			LEFT JOIN nodes n
			  ON n.id = nf.file_id
			 AND n.library_id = nf.library_id
			GROUP BY so.id
		)
		SELECT
			so.provider AS provider,
			so.bucket AS bucket,
			COUNT(*) AS object_count,
			COALESCE(SUM(ref.file_ref_count), 0) AS file_ref_count,
			COALESCE(SUM(so.content_length), 0) AS physical_bytes,
			COUNT(*) FILTER (WHERE COALESCE(ref.has_visible_ref, FALSE)) AS visible_object_count,
			COALESCE(SUM(ref.visible_file_ref_count), 0) AS visible_file_ref_count,
			COALESCE(
				SUM(so.content_length) FILTER (WHERE COALESCE(ref.has_visible_ref, FALSE)),
				0
			) AS visible_bytes,
			COUNT(*) FILTER (
				WHERE NOT COALESCE(ref.has_visible_ref, FALSE)
				  AND COALESCE(ref.has_recycle_ref, FALSE)
			) AS recycle_object_count,
			COALESCE(
				SUM(ref.recycle_file_ref_count) FILTER (
					WHERE NOT COALESCE(ref.has_visible_ref, FALSE)
					  AND COALESCE(ref.has_recycle_ref, FALSE)
				),
				0
			) AS recycle_file_ref_count,
			COALESCE(
				SUM(so.content_length) FILTER (
					WHERE NOT COALESCE(ref.has_visible_ref, FALSE)
					  AND COALESCE(ref.has_recycle_ref, FALSE)
				),
				0
			) AS recycle_bytes,
			COUNT(*) FILTER (WHERE COALESCE(ref.file_ref_count, 0) = 0) AS orphan_object_count,
			COALESCE(
				SUM(so.content_length) FILTER (WHERE COALESCE(ref.file_ref_count, 0) = 0),
				0
			) AS orphan_bytes
		FROM scoped_objects so
		LEFT JOIN object_refs ref
		  ON ref.storage_object_id = so.id
		GROUP BY so.provider, so.bucket
		ORDER BY physical_bytes DESC, object_count DESC, provider ASC, bucket ASC
	`

	var rows []domain.StorageDistributionRow
	if err := r.dbWithContext(ctx).
		Raw(querySQL, int64(ownerUserID), int64(libraryID), int64(libraryID)).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
