package repository

import (
	"context"
	"time"

	domain "omniflow-go/internal/domain/uploadsession"
	pgtx "omniflow-go/internal/repository/postgres/impl/txctx"
	pgmodel "omniflow-go/internal/repository/postgres/model"
	pgquery "omniflow-go/internal/repository/postgres/query"

	"github.com/samber/lo"
	"gorm.io/gorm"
)

type UploadSessionRepository struct {
	db *gorm.DB
}

func NewUploadSessionRepository(db *gorm.DB) *UploadSessionRepository {
	return &UploadSessionRepository{db: db}
}

func (r *UploadSessionRepository) WithTx(tx *gorm.DB) *UploadSessionRepository {
	if tx == nil {
		return r
	}
	return &UploadSessionRepository{db: tx}
}

func (r *UploadSessionRepository) dbWithContext(ctx context.Context) *gorm.DB {
	if tx, ok := pgtx.FromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func (r *UploadSessionRepository) query(ctx context.Context) *pgquery.Query {
	return pgquery.Use(r.dbWithContext(ctx))
}

// CreateInput 是 Init 阶段写入 upload_sessions 行所需的全部字段。
type CreateInput struct {
	ID              string
	LibraryID       uint64
	ParentID        uint64
	ActorID         string
	StorageKey      string
	FileName        string
	FileSize        int64
	ContentType     string
	StorageProvider string
	Mode            string
	MinioUploadID   string
	PartSize        int64
	ExpiresAt       time.Time
}

func (r *UploadSessionRepository) Create(ctx context.Context, input CreateInput) (domain.UploadSession, error) {
	row := &pgmodel.UploadSession{
		ID:              input.ID,
		LibraryID:       int64(input.LibraryID),
		ParentID:        nullableInt64(int64(input.ParentID)),
		ActorID:         input.ActorID,
		StorageKey:      input.StorageKey,
		FileName:        input.FileName,
		FileSize:        input.FileSize,
		ContentType:     input.ContentType,
		StorageProvider: input.StorageProvider,
		Mode:            input.Mode,
		MinioUploadID:   nullableString(input.MinioUploadID),
		PartSize:        input.PartSize,
		ExpiresAt:       input.ExpiresAt,
	}

	q := r.query(ctx)
	if err := q.UploadSession.WithContext(ctx).Create(row); err != nil {
		return domain.UploadSession{}, mapDBError(err)
	}
	return toDomain(row), nil
}

// Get 按 ID 拉取会话。未找到返回 ErrNotFound。
func (r *UploadSessionRepository) Get(ctx context.Context, id string) (domain.UploadSession, error) {
	q := r.query(ctx)
	row, err := q.UploadSession.WithContext(ctx).
		Where(q.UploadSession.ID.Eq(id)).
		First()
	if err != nil {
		return domain.UploadSession{}, mapDBError(err)
	}
	return toDomain(row), nil
}

// UpdateExpiresAt 续约 TTL；同时返回受影响行数，调用方据此判断会话是否仍存在。
func (r *UploadSessionRepository) UpdateExpiresAt(ctx context.Context, id string, expiresAt time.Time) (bool, error) {
	q := r.query(ctx)
	info, err := q.UploadSession.WithContext(ctx).
		Where(q.UploadSession.ID.Eq(id)).
		Updates(map[string]any{
			"expires_at": expiresAt,
			"updated_at": time.Now().UTC(),
		})
	if err != nil {
		return false, err
	}
	return info.RowsAffected > 0, nil
}

func (r *UploadSessionRepository) Delete(ctx context.Context, id string) (bool, error) {
	q := r.query(ctx)
	info, err := q.UploadSession.WithContext(ctx).
		Where(q.UploadSession.ID.Eq(id)).
		Delete()
	if err != nil {
		return false, err
	}
	return info.RowsAffected > 0, nil
}

// ListExpiredBefore 拉出 expires_at <= 截止时间的会话，按 expires_at 升序，limit 限制单批规模。
// Janitor 用此扫描过期会话，逐一调用 MinIO Abort + Delete。
func (r *UploadSessionRepository) ListExpiredBefore(ctx context.Context, before time.Time, limit int) ([]domain.UploadSession, error) {
	if limit <= 0 {
		limit = 100
	}
	q := r.query(ctx)
	rows, err := q.UploadSession.WithContext(ctx).
		Where(q.UploadSession.ExpiresAt.Lte(before)).
		Order(q.UploadSession.ExpiresAt.Asc()).
		Limit(limit).
		Find()
	if err != nil {
		return nil, err
	}
	return lo.Map(rows, func(row *pgmodel.UploadSession, _ int) domain.UploadSession {
		return toDomain(row)
	}), nil
}
