package repository

import (
	"context"
	"time"

	domainlibrary "omniflow-go/internal/domain/library"
	pgtx "omniflow-go/internal/repository/postgres/impl/txctx"
	pgmodel "omniflow-go/internal/repository/postgres/model"
	pgquery "omniflow-go/internal/repository/postgres/query"

	"github.com/samber/lo"
	"gorm.io/gorm"
)

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

func (r *LibraryRepository) dbWithContext(ctx context.Context) *gorm.DB {
	if tx, ok := pgtx.FromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func (r *LibraryRepository) query(ctx context.Context) *pgquery.Query {
	return pgquery.Use(r.dbWithContext(ctx))
}

func (r *LibraryRepository) ScrollByUser(ctx context.Context, userID, lastID uint64, size int) ([]domainlibrary.Library, error) {
	q := r.query(ctx)

	do := q.Library.WithContext(ctx).
		Where(q.Library.UserID.Eq(int64(userID))).
		Order(q.Library.ID.Asc()).
		Limit(size)
	if lastID > 0 {
		do = do.Where(q.Library.ID.Gt(int64(lastID)))
	}

	rows, err := do.Find()
	if err != nil {
		return nil, err
	}

	result := lo.Map(rows, func(item *pgmodel.Library, _ int) domainlibrary.Library {
		return toDomainLibraryModel(item)
	})
	return result, nil
}

func (r *LibraryRepository) Create(ctx context.Context, userID uint64, name string) (domainlibrary.Library, error) {
	row := &pgmodel.Library{
		UserID: nullableInt64(int64(userID)),
		Name:   name,
	}

	q := r.query(ctx)
	if err := q.Library.WithContext(ctx).Create(row); err != nil {
		return domainlibrary.Library{}, err
	}
	return toDomainLibraryModel(row), nil
}

func (r *LibraryRepository) UpdateName(ctx context.Context, id, userID uint64, name string, updatedAt time.Time) (bool, error) {
	return r.UpdateFields(ctx, id, userID, map[string]any{
		"name":       name,
		"updated_at": updatedAt,
	})
}

func (r *LibraryRepository) UpdateFields(ctx context.Context, id, userID uint64, updates map[string]any) (bool, error) {
	q := r.query(ctx)

	info, err := q.Library.WithContext(ctx).
		Where(
			q.Library.ID.Eq(int64(id)),
			q.Library.UserID.Eq(int64(userID)),
		).
		Updates(updates)
	if err != nil {
		return false, err
	}
	return info.RowsAffected > 0, nil
}

func (r *LibraryRepository) SoftDelete(ctx context.Context, id, userID uint64, deletedAt time.Time) (bool, error) {
	q := r.query(ctx)

	info, err := q.Library.WithContext(ctx).
		Where(
			q.Library.ID.Eq(int64(id)),
			q.Library.UserID.Eq(int64(userID)),
		).
		Update(q.Library.DeletedAt, deletedAt)
	if err != nil {
		return false, err
	}
	return info.RowsAffected > 0, nil
}

func (r *LibraryRepository) FindByID(ctx context.Context, id uint64) (domainlibrary.Library, error) {
	q := r.query(ctx)

	row, err := q.Library.WithContext(ctx).
		Where(q.Library.ID.Eq(int64(id))).
		First()
	if err != nil {
		return domainlibrary.Library{}, mapDBError(err)
	}
	return toDomainLibraryModel(row), nil
}
