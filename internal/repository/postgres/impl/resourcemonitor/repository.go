package repository

import (
	"context"
	"errors"

	domain "omniflow-go/internal/domain/resourcemonitor"
	pgmodel "omniflow-go/internal/repository/postgres/model"
	pgquery "omniflow-go/internal/repository/postgres/query"

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

func (r *Repository) query(ctx context.Context) *pgquery.Query {
	return pgquery.Use(r.dbWithContext(ctx))
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

// SaveSample 保存一条资源监测历史采样。
func (r *Repository) SaveSample(ctx context.Context, sample domain.Sample) (domain.Sample, error) {
	if r.db == nil {
		return domain.Sample{}, errors.New("resource monitor repository: database is nil")
	}
	row := sampleToModel(sample)
	q := r.query(ctx)
	if err := q.ResourceMonitorSamples.WithContext(ctx).Create(row); err != nil {
		return domain.Sample{}, err
	}
	return sampleFromModel(row), nil
}

// LibraryBelongsToOwner 校验资料库是否属于指定用户。
func (r *Repository) LibraryBelongsToOwner(ctx context.Context, ownerUserID uint64, libraryID uint64) (bool, error) {
	if r.db == nil {
		return false, errors.New("resource monitor repository: database is nil")
	}
	q := r.query(ctx)
	count, err := q.Library.WithContext(ctx).
		Where(
			q.Library.ID.Eq(int64(libraryID)),
			q.Library.UserID.Eq(int64(ownerUserID)),
		).
		Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
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

	var rows []domain.StorageDistributionRow
	if err := r.dbWithContext(ctx).
		Raw(storageDistributionSQL, int64(ownerUserID), int64(libraryID), int64(libraryID)).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func sampleToModel(sample domain.Sample) *pgmodel.ResourceMonitorSamples {
	return &pgmodel.ResourceMonitorSamples{
		ActorID:             sample.ActorID,
		Scope:               sample.Scope,
		LibraryID:           sample.LibraryID,
		GeneratedAt:         sample.GeneratedAt,
		ProviderCount:       int32(sample.ProviderCount),
		BucketCount:         int32(sample.BucketCount),
		ObjectCount:         sample.ObjectCount,
		FileRefCount:        sample.FileRefCount,
		PhysicalBytes:       sample.PhysicalBytes,
		VisibleObjectCount:  sample.VisibleObjectCount,
		VisibleFileRefCount: sample.VisibleFileRefCount,
		VisibleBytes:        sample.VisibleBytes,
		RecycleObjectCount:  sample.RecycleObjectCount,
		RecycleFileRefCount: sample.RecycleFileRefCount,
		RecycleBytes:        sample.RecycleBytes,
		OrphanObjectCount:   sample.OrphanObjectCount,
		OrphanBytes:         sample.OrphanBytes,
		UnmatchedCount:      int32(sample.UnmatchedCount),
		LegacyProviderCount: int32(sample.LegacyProviderCount),
		ProbeTotal:          int32(sample.ProbeTotal),
		ProbeOk:             int32(sample.ProbeOK),
		ProbeError:          int32(sample.ProbeError),
		ProbeUnknown:        int32(sample.ProbeUnknown),
		DistributionError:   sample.DistributionError,
		SnapshotJSON:        sample.SnapshotJSON,
	}
}

func sampleFromModel(row *pgmodel.ResourceMonitorSamples) domain.Sample {
	if row == nil {
		return domain.Sample{}
	}
	return domain.Sample{
		ID:                  row.ID,
		ActorID:             row.ActorID,
		Scope:               row.Scope,
		LibraryID:           row.LibraryID,
		GeneratedAt:         row.GeneratedAt,
		ProviderCount:       int(row.ProviderCount),
		BucketCount:         int(row.BucketCount),
		ObjectCount:         row.ObjectCount,
		FileRefCount:        row.FileRefCount,
		PhysicalBytes:       row.PhysicalBytes,
		VisibleObjectCount:  row.VisibleObjectCount,
		VisibleFileRefCount: row.VisibleFileRefCount,
		VisibleBytes:        row.VisibleBytes,
		RecycleObjectCount:  row.RecycleObjectCount,
		RecycleFileRefCount: row.RecycleFileRefCount,
		RecycleBytes:        row.RecycleBytes,
		OrphanObjectCount:   row.OrphanObjectCount,
		OrphanBytes:         row.OrphanBytes,
		UnmatchedCount:      int(row.UnmatchedCount),
		LegacyProviderCount: int(row.LegacyProviderCount),
		ProbeTotal:          int(row.ProbeTotal),
		ProbeOK:             int(row.ProbeOk),
		ProbeError:          int(row.ProbeError),
		ProbeUnknown:        int(row.ProbeUnknown),
		DistributionError:   row.DistributionError,
		SnapshotJSON:        row.SnapshotJSON,
		CreatedAt:           row.CreatedAt,
	}
}
