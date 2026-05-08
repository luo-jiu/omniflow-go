// Package repository 提供 migration_tasks / migration_task_items 的 PostgreSQL 仓储实现。
//
// 本包遵循 omniflow-go 通用仓储分层规范：
//   - 常规查询走 GORM 链式 API；
//   - 高并发抢任务依赖 PostgreSQL 的 FOR UPDATE SKIP LOCKED，必须用 raw SQL，
//     集中收敛在 ClaimNextItem 一处，参数化查询。
package repository

import (
	"context"
	"time"

	domain "omniflow-go/internal/domain/migration"
	pgtx "omniflow-go/internal/repository/postgres/impl/txctx"
	pgmodel "omniflow-go/internal/repository/postgres/model"
	pgquery "omniflow-go/internal/repository/postgres/query"

	"github.com/samber/lo"
	"gorm.io/gorm"
)

// MigrationRepository 是 migration_tasks 与 migration_task_items 的统一仓储。
type MigrationRepository struct {
	db *gorm.DB
}

// NewMigrationRepository 构造仓储。
func NewMigrationRepository(db *gorm.DB) *MigrationRepository {
	return &MigrationRepository{db: db}
}

// WithTx 返回绑定到指定事务的仓储副本。
func (r *MigrationRepository) WithTx(tx *gorm.DB) *MigrationRepository {
	if tx == nil {
		return r
	}
	return &MigrationRepository{db: tx}
}

func (r *MigrationRepository) dbWithContext(ctx context.Context) *gorm.DB {
	if tx, ok := pgtx.FromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func (r *MigrationRepository) query(ctx context.Context) *pgquery.Query {
	return pgquery.Use(r.dbWithContext(ctx))
}

// CreateTaskInput 是创建迁移任务所需的字段。
type CreateTaskInput struct {
	ID             string
	ActorID        string
	LibraryID      int64
	RootNodeID     int64
	TargetProvider string
	TotalObjects   int32
	TotalBytes     int64
}

// CreateItemInput 是创建迁移子项所需的字段。
type CreateItemInput struct {
	StorageObjectID int64
	SourceProvider  string
	SourceBucket    string
	SourceKey       string
	TargetKey       string
	FileSize        int64
}

// CreateTaskWithItems 在单个事务中创建任务及其所有子项。
// 上层调用方必须在事务中调用本方法（即外层使用 GormTransactor.WithinTx 包裹），
// 仓储自身不显式开启事务以保持事务边界统一在 usecase 层。
func (r *MigrationRepository) CreateTaskWithItems(
	ctx context.Context,
	task CreateTaskInput,
	items []CreateItemInput,
) (domain.Task, error) {
	now := time.Now().UTC()
	taskRow := &pgmodel.MigrationTask{
		ID:             task.ID,
		ActorID:        task.ActorID,
		LibraryID:      task.LibraryID,
		RootNodeID:     task.RootNodeID,
		TargetProvider: task.TargetProvider,
		Status:         domain.TaskStatusPending,
		TotalObjects:   task.TotalObjects,
		TotalBytes:     task.TotalBytes,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	db := r.dbWithContext(ctx)
	if err := db.Create(taskRow).Error; err != nil {
		return domain.Task{}, mapDBError(err)
	}

	if len(items) > 0 {
		rows := make([]*pgmodel.MigrationTaskItem, 0, len(items))
		for _, it := range items {
			rows = append(rows, &pgmodel.MigrationTaskItem{
				TaskID:          task.ID,
				StorageObjectID: it.StorageObjectID,
				SourceProvider:  it.SourceProvider,
				SourceBucket:    it.SourceBucket,
				SourceKey:       it.SourceKey,
				TargetKey:       it.TargetKey,
				FileSize:        it.FileSize,
				Status:          domain.ItemStatusPending,
				CreatedAt:       now,
			})
		}
		if err := db.CreateInBatches(rows, 200).Error; err != nil {
			return domain.Task{}, mapDBError(err)
		}
	}

	return taskToDomain(taskRow), nil
}

// GetTask 根据任务 ID 取出任务。
func (r *MigrationRepository) GetTask(ctx context.Context, id string) (domain.Task, error) {
	var row pgmodel.MigrationTask
	if err := r.dbWithContext(ctx).
		Where("id = ?", id).
		First(&row).Error; err != nil {
		return domain.Task{}, mapDBError(err)
	}
	return taskToDomain(&row), nil
}

// ListTasksFilter 控制 ListTasks 的过滤条件。
type ListTasksFilter struct {
	ActorID   string
	LibraryID int64
	Statuses  []string
	Limit     int
}

// ListTasks 拉取 actor + library 维度的任务列表，按创建时间降序。
func (r *MigrationRepository) ListTasks(
	ctx context.Context,
	filter ListTasksFilter,
) ([]domain.Task, error) {
	tx := r.dbWithContext(ctx).Model(&pgmodel.MigrationTask{})
	if filter.ActorID != "" {
		tx = tx.Where("actor_id = ?", filter.ActorID)
	}
	if filter.LibraryID > 0 {
		tx = tx.Where("library_id = ?", filter.LibraryID)
	}
	if len(filter.Statuses) > 0 {
		tx = tx.Where("status IN ?", filter.Statuses)
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	var rows []*pgmodel.MigrationTask
	if err := tx.Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return lo.Map(rows, func(row *pgmodel.MigrationTask, _ int) domain.Task {
		return taskToDomain(row)
	}), nil
}

// UpdateTaskStatus 将任务状态切换为目标值。
// finishedAt 仅在切到终态时才会被写入。
func (r *MigrationRepository) UpdateTaskStatus(
	ctx context.Context,
	id, status, errorMessage string,
) (bool, error) {
	now := time.Now().UTC()
	updates := map[string]any{
		"status":     status,
		"updated_at": now,
	}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}
	switch status {
	case domain.TaskStatusRunning:
		updates["started_at"] = now
	case domain.TaskStatusCompleted, domain.TaskStatusFailed, domain.TaskStatusCanceled:
		updates["finished_at"] = now
	}

	res := r.dbWithContext(ctx).
		Model(&pgmodel.MigrationTask{}).
		Where("id = ?", id).
		Updates(updates)
	if res.Error != nil {
		return false, mapDBError(res.Error)
	}
	return res.RowsAffected > 0, nil
}

// IncrementProgress 按增量更新任务的进度计数。
// 计数字段使用 SQL 表达式自增，避免并发覆盖。
func (r *MigrationRepository) IncrementProgress(
	ctx context.Context,
	taskID string,
	delta domain.ProgressDelta,
) error {
	updates := map[string]any{
		"updated_at": time.Now().UTC(),
	}
	if delta.CompletedObjects > 0 {
		updates["completed_objects"] = gorm.Expr("completed_objects + ?", delta.CompletedObjects)
	}
	if delta.FailedObjects > 0 {
		updates["failed_objects"] = gorm.Expr("failed_objects + ?", delta.FailedObjects)
	}
	if delta.SkippedObjects > 0 {
		updates["skipped_objects"] = gorm.Expr("skipped_objects + ?", delta.SkippedObjects)
	}
	if delta.TransferredBytes > 0 {
		updates["transferred_bytes"] = gorm.Expr("transferred_bytes + ?", delta.TransferredBytes)
	}
	if delta.CurrentObjectKey != "" {
		updates["current_object_key"] = delta.CurrentObjectKey
	}
	if len(updates) == 1 {
		return nil
	}
	return r.dbWithContext(ctx).
		Model(&pgmodel.MigrationTask{}).
		Where("id = ?", taskID).
		Updates(updates).Error
}

// AggregateTaskProgressIfFinal 检查任务的所有子项是否都已结束，
// 若是则把任务切到 completed / failed 终态。
// completed: 全部 done 或 done+skipped；failed: 至少一个 failed。
func (r *MigrationRepository) AggregateTaskProgressIfFinal(
	ctx context.Context,
	taskID string,
) (domain.Task, bool, error) {
	type counter struct {
		Total   int64
		Pending int64
		Failed  int64
	}
	var c counter
	if err := r.dbWithContext(ctx).
		Model(&pgmodel.MigrationTaskItem{}).
		Select(
			"COUNT(*) AS total, "+
				"COUNT(*) FILTER (WHERE status IN ('pending','running')) AS pending, "+
				"COUNT(*) FILTER (WHERE status = 'failed') AS failed",
		).
		Where("task_id = ?", taskID).
		Scan(&c).Error; err != nil {
		return domain.Task{}, false, err
	}

	if c.Pending > 0 {
		return domain.Task{}, false, nil
	}

	finalStatus := domain.TaskStatusCompleted
	if c.Failed > 0 {
		finalStatus = domain.TaskStatusFailed
	}

	if _, err := r.UpdateTaskStatus(ctx, taskID, finalStatus, ""); err != nil {
		return domain.Task{}, false, err
	}
	t, err := r.GetTask(ctx, taskID)
	if err != nil {
		return domain.Task{}, false, err
	}
	return t, true, nil
}
