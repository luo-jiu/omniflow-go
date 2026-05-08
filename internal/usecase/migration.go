package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	"omniflow-go/internal/authz"
	"omniflow-go/internal/config"
	domainmigration "omniflow-go/internal/domain/migration"
	"omniflow-go/internal/repository"
	migrationpg "omniflow-go/internal/repository/postgres/impl/migration"
	"omniflow-go/internal/storage"

	"github.com/google/uuid"
)

// EnqueueMigrationCommand 创建迁移任务的入参。
//
//	DryRun=true 时仅跑校验和枚举，返回未落库的 PreviewTask，不会真正建表行；
//	这是 CLI / Web 在确认前给用户展示的"将迁移 X 文件 / Y 字节"占位结果。
type EnqueueMigrationCommand struct {
	Actor          actor.Actor
	LibraryID      int64
	RootNodeID     int64
	TargetProvider string
	DryRun         bool
}

// EnqueueMigrationResult 是 Enqueue 的返回值。
//
//	真实执行：Task 是已落库的任务；
//	DryRun：Task.ID 为空，TotalObjects/TotalBytes 反映将要迁移的规模。
type EnqueueMigrationResult struct {
	DryRun           bool
	Task             domainmigration.Task
	TargetProvider   string
	TargetBucket     string
	PlannedObjects   int32
	PlannedBytes     int64
	SkippedSameProv  int32
	StorageObjectIDs []int64
}

// MigrationUseCase 编排存储迁移业务流程：枚举 → 建任务 → worker 抢任务 → 切引用 → 删源。
type MigrationUseCase struct {
	repo       *repository.MigrationRepository
	registry   *storage.StorageRegistry
	authorizer authz.Authorizer
	auditLog   audit.Sink
	tx         repository.Transactor
	cfg        *config.Config
}

// NewMigrationUseCase 构造迁移 UC。
func NewMigrationUseCase(
	repo *repository.MigrationRepository,
	registry *storage.StorageRegistry,
	authorizer authz.Authorizer,
	auditLog audit.Sink,
	tx repository.Transactor,
	cfg *config.Config,
) *MigrationUseCase {
	return &MigrationUseCase{
		repo:       repo,
		registry:   registry,
		authorizer: authorizer,
		auditLog:   auditLog,
		tx:         tx,
		cfg:        cfg,
	}
}

// Enqueue 创建迁移任务。
func (u *MigrationUseCase) Enqueue(ctx context.Context, cmd EnqueueMigrationCommand) (EnqueueMigrationResult, error) {
	if u.repo == nil || u.registry == nil {
		return EnqueueMigrationResult{}, fmt.Errorf("%w: migration not configured", ErrInvalidArgument)
	}
	if cmd.LibraryID <= 0 || cmd.RootNodeID <= 0 {
		return EnqueueMigrationResult{}, fmt.Errorf("%w: libraryId and rootNodeId are required", ErrInvalidArgument)
	}
	target := strings.TrimSpace(cmd.TargetProvider)
	if target == "" {
		return EnqueueMigrationResult{}, fmt.Errorf("%w: targetProvider is required", ErrInvalidArgument)
	}

	if err := u.authorize(ctx, cmd.Actor, uint64(cmd.LibraryID), authz.ActionWrite); err != nil {
		return EnqueueMigrationResult{}, err
	}

	targetStore, err := u.registry.Get(target)
	if err != nil {
		return EnqueueMigrationResult{}, fmt.Errorf("%w: storage provider %q: %v", ErrInvalidArgument, target, err)
	}

	// 直传 MinIO 链路 / node 创建链路写入 storage_objects.provider 用的是 alias（如 win-minio / local-minio），
	// 而非旧的 type 值（MINIO）。迁移侧必须保持同样口径，否则新插入的行会被认成"未知 provider"导致后续无法解析。
	refs, err := u.repo.EnumerateStorageObjectsUnderNode(ctx, cmd.LibraryID, cmd.RootNodeID, target)
	if err != nil {
		return EnqueueMigrationResult{}, fmt.Errorf("enumerate storage objects: %w", err)
	}

	totalObjects := int32(len(refs))
	var totalBytes int64
	storageIDs := make([]int64, 0, len(refs))
	items := make([]migrationpg.CreateItemInput, 0, len(refs))
	for _, ref := range refs {
		totalBytes += ref.ContentLength
		storageIDs = append(storageIDs, ref.StorageObjectID)
		ext := path.Ext(ref.ObjectKey)
		targetKey := fmt.Sprintf("libraries/%d/%s%s", cmd.LibraryID, uuid.NewString(), ext)
		items = append(items, migrationpg.CreateItemInput{
			StorageObjectID: ref.StorageObjectID,
			SourceProvider:  ref.Provider,
			SourceBucket:    ref.Bucket,
			SourceKey:       ref.ObjectKey,
			TargetKey:       targetKey,
			FileSize:        ref.ContentLength,
		})
	}

	if cmd.DryRun {
		_ = u.writeAudit(ctx, cmd.Actor, "migration.enqueue", true, map[string]any{
			"mode":            "dry-run",
			"library_id":      cmd.LibraryID,
			"root_node_id":    cmd.RootNodeID,
			"target_provider": target,
			"planned_objects": totalObjects,
			"planned_bytes":   totalBytes,
		})
		return EnqueueMigrationResult{
			DryRun: true,
			Task: domainmigration.Task{
				ActorID:        cmd.Actor.ID,
				LibraryID:      cmd.LibraryID,
				RootNodeID:     cmd.RootNodeID,
				TargetProvider: target,
				Status:         domainmigration.TaskStatusPending,
				TotalObjects:   totalObjects,
				TotalBytes:     totalBytes,
			},
			TargetProvider:   target,
			TargetBucket:     targetStore.Bucket(),
			PlannedObjects:   totalObjects,
			PlannedBytes:     totalBytes,
			StorageObjectIDs: storageIDs,
		}, nil
	}

	if totalObjects == 0 {
		return EnqueueMigrationResult{}, fmt.Errorf("%w: no storage objects to migrate (already on %s or empty)", ErrInvalidArgument, target)
	}

	taskID := uuid.NewString()
	task, err := u.repo.CreateTaskWithItems(ctx, migrationpg.CreateTaskInput{
		ID:             taskID,
		ActorID:        cmd.Actor.ID,
		LibraryID:      cmd.LibraryID,
		RootNodeID:     cmd.RootNodeID,
		TargetProvider: target,
		TotalObjects:   totalObjects,
		TotalBytes:     totalBytes,
	}, items)
	if err != nil {
		return EnqueueMigrationResult{}, fmt.Errorf("create migration task: %w", err)
	}

	_ = u.writeAudit(ctx, cmd.Actor, "migration.enqueue", true, map[string]any{
		"mode":            "execute",
		"task_id":         taskID,
		"library_id":      cmd.LibraryID,
		"root_node_id":    cmd.RootNodeID,
		"target_provider": target,
		"total_objects":   totalObjects,
		"total_bytes":     totalBytes,
	})
	slog.InfoContext(ctx, "migration.enqueue",
		"task_id", taskID,
		"library_id", cmd.LibraryID,
		"target_provider", target,
		"total_objects", totalObjects,
		"total_bytes", totalBytes,
	)

	return EnqueueMigrationResult{
		Task:             task,
		TargetProvider:   target,
		TargetBucket:     targetStore.Bucket(),
		PlannedObjects:   totalObjects,
		PlannedBytes:     totalBytes,
		StorageObjectIDs: storageIDs,
	}, nil
}

// Cancel 取消迁移任务：状态置 canceled，剩余 pending 子项一次性置 skipped。
// 已 running 子项让 worker 自然结束；已 done 子项不回滚。
func (u *MigrationUseCase) Cancel(ctx context.Context, principal actor.Actor, taskID string) error {
	task, err := u.loadTask(ctx, principal, taskID)
	if err != nil {
		return err
	}
	if task.IsTerminal() {
		return fmt.Errorf("%w: task already terminal: %s", ErrConflict, task.Status)
	}

	if _, err := u.repo.UpdateTaskStatus(ctx, task.ID, domainmigration.TaskStatusCanceled, "canceled by user"); err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	if _, err := u.repo.SkipPendingItemsForTask(ctx, task.ID, "task canceled"); err != nil {
		slog.WarnContext(ctx, "migration.cancel.skip_pending_failed", "task_id", task.ID, "error", err)
	}

	_ = u.writeAudit(ctx, principal, "migration.cancel", true, map[string]any{
		"task_id":    task.ID,
		"library_id": task.LibraryID,
	})
	slog.InfoContext(ctx, "migration.cancel", "task_id", task.ID, "library_id", task.LibraryID)
	return nil
}

// GetTask 查询单个任务（actor 校验）。
func (u *MigrationUseCase) GetTask(ctx context.Context, principal actor.Actor, taskID string) (domainmigration.Task, error) {
	return u.loadTask(ctx, principal, taskID)
}

// ListTasksFilter 与 repo 同名结构对齐，便于 handler 透传。
type ListTasksFilter = migrationpg.ListTasksFilter

// ListTasks 列出 actor 在指定 library 下的迁移任务。
func (u *MigrationUseCase) ListTasks(ctx context.Context, principal actor.Actor, filter ListTasksFilter) ([]domainmigration.Task, error) {
	if filter.LibraryID > 0 {
		if err := u.authorize(ctx, principal, uint64(filter.LibraryID), authz.ActionRead); err != nil {
			return nil, err
		}
	}
	filter.ActorID = principal.ID
	return u.repo.ListTasks(ctx, filter)
}

// ListTaskItems 列出任务下的子项。
func (u *MigrationUseCase) ListTaskItems(ctx context.Context, principal actor.Actor, taskID string) ([]domainmigration.TaskItem, error) {
	if _, err := u.loadTask(ctx, principal, taskID); err != nil {
		return nil, err
	}
	return u.repo.ListItemsByTask(ctx, taskID)
}

// StorageDistribution 给迁移 dialog 返回当前节点子树的 provider 分布。
func (u *MigrationUseCase) StorageDistribution(ctx context.Context, principal actor.Actor, libraryID, rootNodeID int64) ([]migrationpg.ProviderDistribution, error) {
	if libraryID <= 0 || rootNodeID <= 0 {
		return nil, fmt.Errorf("%w: libraryId and rootNodeId are required", ErrInvalidArgument)
	}
	if err := u.authorize(ctx, principal, uint64(libraryID), authz.ActionRead); err != nil {
		return nil, err
	}
	return u.repo.CountStorageDistribution(ctx, libraryID, rootNodeID)
}

// loadTask 拉取任务并做 actor 校验，actor 不一致按 ErrNotFound 返回防枚举。
func (u *MigrationUseCase) loadTask(ctx context.Context, principal actor.Actor, taskID string) (domainmigration.Task, error) {
	id := strings.TrimSpace(taskID)
	if id == "" {
		return domainmigration.Task{}, fmt.Errorf("%w: taskId is required", ErrInvalidArgument)
	}
	task, err := u.repo.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domainmigration.Task{}, fmt.Errorf("%w: migration task not found", ErrNotFound)
		}
		return domainmigration.Task{}, err
	}
	if task.ActorID != principal.ID {
		return domainmigration.Task{}, fmt.Errorf("%w: migration task not found", ErrNotFound)
	}
	return task, nil
}

func (u *MigrationUseCase) authorize(ctx context.Context, principal actor.Actor, libraryID uint64, action authz.Action) error {
	if u.authorizer == nil {
		return nil
	}
	return u.authorizer.Authorize(ctx, principal, authz.Resource{
		Kind: "library",
		ID:   fmt.Sprintf("%d", libraryID),
	}, action)
}

func (u *MigrationUseCase) writeAudit(ctx context.Context, principal actor.Actor, action string, success bool, metadata map[string]any) error {
	if u.auditLog == nil {
		return nil
	}
	return u.auditLog.Write(ctx, audit.Event{
		Actor:      principal,
		Action:     action,
		Resource:   "migration_task",
		Success:    success,
		OccurredAt: time.Now().UTC(),
		Metadata:   metadata,
	})
}

// processItem 是 worker 单次处理一个子项的核心逻辑。
//
// 流程：复制 → 校验 → 在事务内交换 storage_object 引用 → 标记 done → 异步删源（best effort）。
// 任何步骤失败都会标记 item failed 并把错误写到 error_message，不影响其他子项。
func (u *MigrationUseCase) processItem(ctx context.Context, item domainmigration.TaskItem) error {
	task, err := u.repo.GetTask(ctx, item.TaskID)
	if err != nil {
		// task 找不到属于异常状态，子项无法继续处理。
		_ = u.repo.MarkItemFailed(ctx, item.ID, fmt.Sprintf("get task: %v", err))
		return err
	}
	if task.IsTerminal() {
		// 任务已被取消或已结束：把 running 子项也置为 skipped，不真实搬运。
		_ = u.repo.MarkItemSkipped(ctx, item.ID, fmt.Sprintf("task already %s", task.Status))
		_ = u.repo.IncrementProgress(ctx, item.TaskID, domainmigration.ProgressDelta{SkippedObjects: 1})
		return nil
	}

	// 1. 准备 source / target store。
	sourceStore, err := u.registry.Get(item.SourceProvider)
	if err != nil {
		return u.failItem(ctx, item, fmt.Errorf("source provider %q: %w", item.SourceProvider, err))
	}
	targetStore, err := u.registry.Get(task.TargetProvider)
	if err != nil {
		return u.failItem(ctx, item, fmt.Errorf("target provider %q: %w", task.TargetProvider, err))
	}

	// 2. 流式复制：source.GetObject → target.Upload。
	srcReader, srcInfo, err := sourceStore.GetObject(ctx, item.SourceKey)
	if err != nil {
		return u.failItem(ctx, item, fmt.Errorf("get source object: %w", err))
	}
	defer func() { _ = srcReader.Close() }()

	contentType := srcInfo.ContentType
	if contentType == "" {
		contentType = defaultUploadContentType
	}
	if err := targetStore.Upload(ctx, item.TargetKey, srcReader, srcInfo.Size, contentType); err != nil {
		return u.failItem(ctx, item, fmt.Errorf("upload to target: %w", err))
	}

	// 3. 校验：HEAD 目标对象，比对大小是否一致。
	targetInfo, err := targetStore.StatObject(ctx, item.TargetKey)
	if err != nil {
		_ = targetStore.Delete(context.Background(), item.TargetKey)
		return u.failItem(ctx, item, fmt.Errorf("stat target object: %w", err))
	}
	if targetInfo.Size != item.FileSize {
		_ = targetStore.Delete(context.Background(), item.TargetKey)
		return u.failItem(ctx, item, fmt.Errorf("target size %d != expected %d", targetInfo.Size, item.FileSize))
	}

	// 4. 事务：插入新 storage_object → 切 node_files 引用 → 软删旧 storage_object → 标记 item done → 任务进度自增。
	var newStorageObjectID int64
	err = u.tx.WithinTx(ctx, func(txCtx context.Context) error {
		swapID, swapErr := u.repo.SwapStorageObject(txCtx, migrationpg.SwapInput{
			OldStorageObjectID: item.StorageObjectID,
			LibraryID:          task.LibraryID,
			TargetProvider:     task.TargetProvider,
			TargetBucket:       targetStore.Bucket(),
			TargetKey:          item.TargetKey,
			ContentLength:      targetInfo.Size,
			ContentType:        targetInfo.ContentType,
			ETag:               targetInfo.ETag,
		})
		if swapErr != nil {
			return swapErr
		}
		newStorageObjectID = swapID
		if err := u.repo.MarkItemDone(txCtx, item.ID, swapID); err != nil {
			return err
		}
		return u.repo.IncrementProgress(txCtx, task.ID, domainmigration.ProgressDelta{
			CompletedObjects: 1,
			TransferredBytes: item.FileSize,
			CurrentObjectKey: item.TargetKey,
		})
	})
	if err != nil {
		// 事务回滚：尝试回收已上传的目标对象，避免孤儿。子项标记 failed。
		_ = targetStore.Delete(context.Background(), item.TargetKey)
		return u.failItem(ctx, item, fmt.Errorf("swap storage object: %w", err))
	}

	// 5. 异步删源（best effort，失败仅 warn）。v1 立即删源策略，孤儿等 v1.1 sweeper 收尾。
	if err := sourceStore.Delete(context.Background(), item.SourceKey); err != nil {
		slog.WarnContext(ctx, "migration.item.source_delete_failed",
			"task_id", task.ID,
			"item_id", item.ID,
			"source_provider", item.SourceProvider,
			"source_key", item.SourceKey,
			"error", err,
		)
	}

	_ = u.writeAudit(ctx, actor.System("migration-worker"), "migration.item.done", true, map[string]any{
		"task_id":               task.ID,
		"item_id":               item.ID,
		"library_id":            task.LibraryID,
		"old_storage_object_id": item.StorageObjectID,
		"new_storage_object_id": newStorageObjectID,
		"source_provider":       item.SourceProvider,
		"target_provider":       task.TargetProvider,
		"target_key":            item.TargetKey,
		"file_size":             item.FileSize,
	})
	slog.InfoContext(ctx, "migration.item.done",
		"task_id", task.ID,
		"item_id", item.ID,
		"new_storage_object_id", newStorageObjectID,
	)

	// 6. 触发任务级聚合检查（所有子项已结束就把任务切到 completed/failed）。
	if _, _, err := u.repo.AggregateTaskProgressIfFinal(ctx, task.ID); err != nil {
		slog.WarnContext(ctx, "migration.aggregate_failed", "task_id", task.ID, "error", err)
	}
	return nil
}

func (u *MigrationUseCase) failItem(ctx context.Context, item domainmigration.TaskItem, cause error) error {
	msg := cause.Error()
	if err := u.repo.MarkItemFailed(ctx, item.ID, msg); err != nil {
		slog.WarnContext(ctx, "migration.mark_failed_error", "item_id", item.ID, "error", err)
	}
	if err := u.repo.IncrementProgress(ctx, item.TaskID, domainmigration.ProgressDelta{
		FailedObjects: 1,
	}); err != nil {
		slog.WarnContext(ctx, "migration.increment_failed_error", "task_id", item.TaskID, "error", err)
	}
	if _, _, aggErr := u.repo.AggregateTaskProgressIfFinal(ctx, item.TaskID); aggErr != nil {
		slog.WarnContext(ctx, "migration.aggregate_failed", "task_id", item.TaskID, "error", aggErr)
	}
	slog.WarnContext(ctx, "migration.item.failed",
		"task_id", item.TaskID,
		"item_id", item.ID,
		"source_key", item.SourceKey,
		"target_key", item.TargetKey,
		"error", msg,
	)
	return cause
}
