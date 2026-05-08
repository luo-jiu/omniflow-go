package repository

import (
	"context"
	"errors"
	"time"

	domain "omniflow-go/internal/domain/migration"
	pgmodel "omniflow-go/internal/repository/postgres/model"

	"github.com/samber/lo"
	"gorm.io/gorm"
)

// ClaimNextItem 用 FOR UPDATE SKIP LOCKED 抢一个 pending 子项并置为 running。
//
// 这里使用 raw SQL 是因为 PostgreSQL 的 SKIP LOCKED 子句在 gorm.io/gen 没有原生表达，
// 强行套链式写法既绕又容易写错。集中收敛在本方法、参数化、注释解释原因，
// 符合 AGENTS.md "递归 / 复杂聚合或确实更清楚的少数场景" 豁免条款。
//
// 行为：
//   - 找出最早的 status='pending' 子项，加排他锁；
//   - 若被其他 worker 锁住则跳过，找下一条；
//   - 没有可领的子项时返回 (zero, false, nil)；
//   - 抢到后将其状态置为 running 并写 started_at，返回更新后的子项。
func (r *MigrationRepository) ClaimNextItem(ctx context.Context) (domain.TaskItem, bool, error) {
	const claimSQL = `
		UPDATE migration_task_items AS target
		SET status = 'running', started_at = ?
		FROM (
			SELECT id
			FROM migration_task_items
			WHERE status = 'pending'
			ORDER BY id
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		) AS picked
		WHERE target.id = picked.id
		RETURNING target.id, target.task_id, target.storage_object_id,
		          target.source_provider, target.source_bucket, target.source_key,
		          target.target_storage_object_id, target.target_key,
		          target.file_size, target.status, target.error_message,
		          target.started_at, target.finished_at, target.created_at
	`
	var row pgmodel.MigrationTaskItem
	now := time.Now().UTC()
	if err := r.dbWithContext(ctx).
		Raw(claimSQL, now).
		Scan(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.TaskItem{}, false, nil
		}
		return domain.TaskItem{}, false, err
	}
	if row.ID == 0 {
		return domain.TaskItem{}, false, nil
	}
	return itemToDomain(&row), true, nil
}

// MarkItemDone 把子项置为 done 并填 target_storage_object_id。
func (r *MigrationRepository) MarkItemDone(
	ctx context.Context,
	itemID, targetStorageObjectID int64,
) error {
	now := time.Now().UTC()
	return r.dbWithContext(ctx).
		Model(&pgmodel.MigrationTaskItem{}).
		Where("id = ?", itemID).
		Updates(map[string]any{
			"status":                   domain.ItemStatusDone,
			"target_storage_object_id": nullableInt64Ptr(targetStorageObjectID),
			"finished_at":              now,
			"error_message":            "",
		}).Error
}

// MarkItemFailed 把子项置为 failed 并写错误信息。
func (r *MigrationRepository) MarkItemFailed(
	ctx context.Context,
	itemID int64,
	errorMessage string,
) error {
	now := time.Now().UTC()
	return r.dbWithContext(ctx).
		Model(&pgmodel.MigrationTaskItem{}).
		Where("id = ?", itemID).
		Updates(map[string]any{
			"status":        domain.ItemStatusFailed,
			"finished_at":   now,
			"error_message": errorMessage,
		}).Error
}

// MarkItemSkipped 把子项置为 skipped。
func (r *MigrationRepository) MarkItemSkipped(
	ctx context.Context,
	itemID int64,
	reason string,
) error {
	now := time.Now().UTC()
	return r.dbWithContext(ctx).
		Model(&pgmodel.MigrationTaskItem{}).
		Where("id = ?", itemID).
		Updates(map[string]any{
			"status":        domain.ItemStatusSkipped,
			"finished_at":   now,
			"error_message": reason,
		}).Error
}

// SkipPendingItemsForTask 任务被取消时把剩余 pending 子项一次性置为 skipped。
func (r *MigrationRepository) SkipPendingItemsForTask(
	ctx context.Context,
	taskID, reason string,
) (int64, error) {
	now := time.Now().UTC()
	res := r.dbWithContext(ctx).
		Model(&pgmodel.MigrationTaskItem{}).
		Where("task_id = ? AND status = ?", taskID, domain.ItemStatusPending).
		Updates(map[string]any{
			"status":        domain.ItemStatusSkipped,
			"finished_at":   now,
			"error_message": reason,
		})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// ListItemsByTask 列出任务下所有子项。
func (r *MigrationRepository) ListItemsByTask(
	ctx context.Context,
	taskID string,
) ([]domain.TaskItem, error) {
	var rows []*pgmodel.MigrationTaskItem
	if err := r.dbWithContext(ctx).
		Where("task_id = ?", taskID).
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return lo.Map(rows, func(row *pgmodel.MigrationTaskItem, _ int) domain.TaskItem {
		return itemToDomain(row)
	}), nil
}

// GetItem 拉取单个子项。
func (r *MigrationRepository) GetItem(ctx context.Context, itemID int64) (domain.TaskItem, error) {
	var row pgmodel.MigrationTaskItem
	if err := r.dbWithContext(ctx).
		Where("id = ?", itemID).
		First(&row).Error; err != nil {
		return domain.TaskItem{}, mapDBError(err)
	}
	return itemToDomain(&row), nil
}
