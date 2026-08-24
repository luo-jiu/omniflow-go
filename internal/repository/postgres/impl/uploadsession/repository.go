package repository

import (
	"context"
	"time"

	domain "omniflow-go/internal/domain/uploadsession"
	pgtx "omniflow-go/internal/repository/postgres/impl/txctx"
	pgmodel "omniflow-go/internal/repository/postgres/model"
	pgquery "omniflow-go/internal/repository/postgres/query"

	"github.com/samber/lo"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
		Status:          domain.StatusPending,
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

// GetForUpdate 在当前事务内锁定会话，供 complete 的 operation 认领与提交状态迁移使用。
func (r *UploadSessionRepository) GetForUpdate(ctx context.Context, id string) (domain.UploadSession, error) {
	q := r.query(ctx)
	row, err := q.UploadSession.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(q.UploadSession.ID.Eq(id)).
		First()
	if err != nil {
		return domain.UploadSession{}, mapDBError(err)
	}
	return toDomain(row), nil
}

// GetByOperationIDAndActor 查询 actor 自己的完成操作；未命中统一返回 ErrNotFound，防止跨 actor 枚举。
func (r *UploadSessionRepository) GetByOperationIDAndActor(
	ctx context.Context,
	actorID string,
	operationID string,
) (domain.UploadSession, error) {
	q := r.query(ctx)
	row, err := q.UploadSession.WithContext(ctx).
		Where(
			q.UploadSession.ActorID.Eq(actorID),
			q.UploadSession.ClientOperationID.Eq(operationID),
		).
		First()
	if err != nil {
		return domain.UploadSession{}, mapDBError(err)
	}
	return toDomain(row), nil
}

// ClaimOperation 将稳定 operation ID 绑定到尚未认领的 pending 会话，并更新本次操作的租约。
func (r *UploadSessionRepository) ClaimOperation(
	ctx context.Context,
	id string,
	operationID string,
	expiresAt time.Time,
) (bool, error) {
	q := r.query(ctx)
	info, err := q.UploadSession.WithContext(ctx).
		Where(
			q.UploadSession.ID.Eq(id),
			q.UploadSession.Status.Eq(domain.StatusPending),
			q.UploadSession.ClientOperationID.IsNull(),
		).
		Updates(map[string]any{
			"client_operation_id": operationID,
			"expires_at":          expiresAt,
			"updated_at":          time.Now().UTC(),
		})
	if err != nil {
		return false, mapDBError(err)
	}
	return info.RowsAffected > 0, nil
}

// ClaimExpiredForCleanup 原子认领已过期 pending 会话，避免 janitor 与 complete/abort 并发清理同一对象。
func (r *UploadSessionRepository) ClaimExpiredForCleanup(
	ctx context.Context,
	id string,
	before time.Time,
	operationID string,
	expiresAt time.Time,
) (bool, error) {
	q := r.query(ctx)
	info, err := q.UploadSession.WithContext(ctx).
		Where(
			q.UploadSession.ID.Eq(id),
			q.UploadSession.Status.Eq(domain.StatusPending),
			q.UploadSession.ExpiresAt.Lte(before),
		).
		Updates(map[string]any{
			"client_operation_id": operationID,
			"expires_at":          expiresAt,
			"updated_at":          time.Now().UTC(),
		})
	if err != nil {
		return false, mapDBError(err)
	}
	return info.RowsAffected > 0, nil
}

// ClearClientOperationID 释放明确失败的 operation 认领；committed 行不会被改写。
func (r *UploadSessionRepository) ClearClientOperationID(
	ctx context.Context,
	id string,
	operationID string,
) (bool, error) {
	q := r.query(ctx)
	info, err := q.UploadSession.WithContext(ctx).
		Where(
			q.UploadSession.ID.Eq(id),
			q.UploadSession.Status.Eq(domain.StatusPending),
			q.UploadSession.ClientOperationID.Eq(operationID),
		).
		Updates(map[string]any{
			"client_operation_id": nil,
			"updated_at":          time.Now().UTC(),
		})
	if err != nil {
		return false, mapDBError(err)
	}
	return info.RowsAffected > 0, nil
}

// MarkCommitted 把 pending 会话转为可重放的完成回执。调用方必须在 node 创建所在事务中执行。
func (r *UploadSessionRepository) MarkCommitted(
	ctx context.Context,
	id string,
	operationID string,
	nodeID uint64,
	completionResult string,
	completedAt time.Time,
	expiresAt time.Time,
) (bool, error) {
	q := r.query(ctx)
	info, err := q.UploadSession.WithContext(ctx).
		Where(
			q.UploadSession.ID.Eq(id),
			q.UploadSession.Status.Eq(domain.StatusPending),
			q.UploadSession.ClientOperationID.Eq(operationID),
		).
		Updates(map[string]any{
			"status":            domain.StatusCommitted,
			"completed_node_id": int64(nodeID),
			"completion_result": datatypes.JSON([]byte(completionResult)),
			"completed_at":      completedAt,
			"expires_at":        expiresAt,
			"updated_at":        completedAt,
		})
	if err != nil {
		return false, mapDBError(err)
	}
	return info.RowsAffected > 0, nil
}

// UpdateExpiresAt 续约 TTL；同时返回受影响行数，调用方据此判断会话是否仍存在。
func (r *UploadSessionRepository) UpdateExpiresAt(ctx context.Context, id string, expiresAt time.Time) (bool, error) {
	q := r.query(ctx)
	info, err := q.UploadSession.WithContext(ctx).
		Where(
			q.UploadSession.ID.Eq(id),
			q.UploadSession.Status.Eq(domain.StatusPending),
			q.UploadSession.ClientOperationID.IsNull(),
		).
		Updates(map[string]any{
			"expires_at": expiresAt,
			"updated_at": time.Now().UTC(),
		})
	if err != nil {
		return false, err
	}
	return info.RowsAffected > 0, nil
}

// DeleteClaimedPending 删除由指定内部操作认领的 pending 会话。
func (r *UploadSessionRepository) DeleteClaimedPending(
	ctx context.Context,
	id string,
	operationID string,
) (bool, error) {
	q := r.query(ctx)
	info, err := q.UploadSession.WithContext(ctx).
		Where(
			q.UploadSession.ID.Eq(id),
			q.UploadSession.Status.Eq(domain.StatusPending),
			q.UploadSession.ClientOperationID.Eq(operationID),
		).
		Delete()
	if err != nil {
		return false, err
	}
	return info.RowsAffected > 0, nil
}

// DeleteExpiredCommitted 删除过期完成回执，不影响任何已提交对象或 node。
func (r *UploadSessionRepository) DeleteExpiredCommitted(
	ctx context.Context,
	id string,
	before time.Time,
) (bool, error) {
	q := r.query(ctx)
	info, err := q.UploadSession.WithContext(ctx).
		Where(
			q.UploadSession.ID.Eq(id),
			q.UploadSession.Status.Eq(domain.StatusCommitted),
			q.UploadSession.ExpiresAt.Lte(before),
		).
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
