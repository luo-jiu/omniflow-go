package handler

import (
	"strings"

	domainmigration "omniflow-go/internal/domain/migration"
	migrationpg "omniflow-go/internal/repository/postgres/impl/migration"
	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

// MigrationHandler 暴露存储迁移的 HTTP 入口。
type MigrationHandler struct {
	uc *usecase.MigrationUseCase
}

// NewMigrationHandler 构造迁移 handler。
func NewMigrationHandler(uc *usecase.MigrationUseCase) *MigrationHandler {
	return &MigrationHandler{uc: uc}
}

type enqueueMigrationRequest struct {
	LibraryID      int64  `json:"libraryId" binding:"required"`
	RootNodeID     int64  `json:"rootNodeId" binding:"required"`
	TargetProvider string `json:"targetProvider" binding:"required"`
}

type listMigrationTasksQuery struct {
	LibraryID int64  `form:"libraryId"`
	Status    string `form:"status"`
	Limit     int    `form:"limit"`
}

type migrationTaskIDURI struct {
	ID string `uri:"id" binding:"required"`
}

type storageDistributionQuery struct {
	NodeID int64 `form:"nodeId" binding:"required"`
}

type libraryIDURIInt struct {
	LibraryID int64 `uri:"libraryId" binding:"required"`
}

// taskResponse 是任务的对外响应结构（避免直接暴露 domain 字段命名）。
type taskResponse struct {
	ID               string  `json:"id"`
	ActorID          string  `json:"actorId"`
	LibraryID        int64   `json:"libraryId"`
	RootNodeID       int64   `json:"rootNodeId"`
	TargetProvider   string  `json:"targetProvider"`
	Status           string  `json:"status"`
	TotalObjects     int32   `json:"totalObjects"`
	CompletedObjects int32   `json:"completedObjects"`
	FailedObjects    int32   `json:"failedObjects"`
	SkippedObjects   int32   `json:"skippedObjects"`
	TotalBytes       int64   `json:"totalBytes"`
	TransferredBytes int64   `json:"transferredBytes"`
	CurrentObjectKey string  `json:"currentObjectKey"`
	ErrorMessage     string  `json:"errorMessage"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
	StartedAt        *string `json:"startedAt"`
	FinishedAt       *string `json:"finishedAt"`
}

type taskItemResponse struct {
	ID                    int64   `json:"id"`
	TaskID                string  `json:"taskId"`
	StorageObjectID       int64   `json:"storageObjectId"`
	SourceProvider        string  `json:"sourceProvider"`
	SourceBucket          string  `json:"sourceBucket"`
	SourceKey             string  `json:"sourceKey"`
	TargetStorageObjectID int64   `json:"targetStorageObjectId"`
	TargetKey             string  `json:"targetKey"`
	FileSize              int64   `json:"fileSize"`
	Status                string  `json:"status"`
	ErrorMessage          string  `json:"errorMessage"`
	StartedAt             *string `json:"startedAt"`
	FinishedAt            *string `json:"finishedAt"`
	CreatedAt             string  `json:"createdAt"`
}

// EnqueueTask POST /api/v1/migration/tasks
func (h *MigrationHandler) EnqueueTask(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var req enqueueMigrationRequest
	if !BindJSON(ctx, &req) {
		return
	}

	if h.uc == nil {
		InternalError(ctx, "migration service not configured")
		return
	}

	result, err := h.uc.Enqueue(ctx.Request.Context(), usecase.EnqueueMigrationCommand{
		Actor:          actorFromContext(ctx),
		LibraryID:      req.LibraryID,
		RootNodeID:     req.RootNodeID,
		TargetProvider: strings.TrimSpace(req.TargetProvider),
		DryRun:         dryRun,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}

	if dryRun {
		SuccessWithDryRun(ctx, true, gin.H{
			"plannedObjects":   result.PlannedObjects,
			"plannedBytes":     result.PlannedBytes,
			"targetProvider":   result.TargetProvider,
			"targetBucket":     result.TargetBucket,
			"storageObjectIds": result.StorageObjectIDs,
		})
		return
	}
	Success(ctx, gin.H{
		"task":             toTaskResponse(result.Task),
		"plannedObjects":   result.PlannedObjects,
		"plannedBytes":     result.PlannedBytes,
		"targetProvider":   result.TargetProvider,
		"targetBucket":     result.TargetBucket,
		"storageObjectIds": result.StorageObjectIDs,
	})
}

// ListTasks GET /api/v1/migration/tasks?libraryId=N[&status=running,pending]
func (h *MigrationHandler) ListTasks(ctx *gin.Context) {
	var query listMigrationTasksQuery
	if !BindQuery(ctx, &query) {
		return
	}

	if h.uc == nil {
		InternalError(ctx, "migration service not configured")
		return
	}

	statuses := splitCommaList(query.Status)
	tasks, err := h.uc.ListTasks(ctx.Request.Context(), actorFromContext(ctx), usecase.ListTasksFilter{
		LibraryID: query.LibraryID,
		Statuses:  statuses,
		Limit:     query.Limit,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}

	out := make([]taskResponse, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, toTaskResponse(t))
	}
	Success(ctx, gin.H{"tasks": out})
}

// GetTask GET /api/v1/migration/tasks/:id
func (h *MigrationHandler) GetTask(ctx *gin.Context) {
	var uri migrationTaskIDURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.uc == nil {
		InternalError(ctx, "migration service not configured")
		return
	}

	task, err := h.uc.GetTask(ctx.Request.Context(), actorFromContext(ctx), uri.ID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, gin.H{"task": toTaskResponse(task)})
}

// CancelTask POST /api/v1/migration/tasks/:id/cancel
func (h *MigrationHandler) CancelTask(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var uri migrationTaskIDURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.uc == nil {
		InternalError(ctx, "migration service not configured")
		return
	}

	if dryRun {
		// dry-run 仅校验任务可取消（actor 校验 + 非终态），不真正落库。
		task, err := h.uc.GetTask(ctx.Request.Context(), actorFromContext(ctx), uri.ID)
		if err != nil {
			HandleUseCaseError(ctx, err)
			return
		}
		if task.IsTerminal() {
			BadRequest(ctx, "task already terminal: "+task.Status)
			return
		}
		SuccessNoDataWithDryRun(ctx, true)
		return
	}

	if err := h.uc.Cancel(ctx.Request.Context(), actorFromContext(ctx), uri.ID); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// ListTaskItems GET /api/v1/migration/tasks/:id/items
func (h *MigrationHandler) ListTaskItems(ctx *gin.Context) {
	var uri migrationTaskIDURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.uc == nil {
		InternalError(ctx, "migration service not configured")
		return
	}

	items, err := h.uc.ListTaskItems(ctx.Request.Context(), actorFromContext(ctx), uri.ID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	out := make([]taskItemResponse, 0, len(items))
	for _, it := range items {
		out = append(out, toTaskItemResponse(it))
	}
	Success(ctx, gin.H{"items": out})
}

// StorageDistribution GET /api/v1/libraries/:libraryId/storage-distribution?nodeId=N
func (h *MigrationHandler) StorageDistribution(ctx *gin.Context) {
	var uri libraryIDURIInt
	if !BindURI(ctx, &uri) {
		return
	}
	var query storageDistributionQuery
	if !BindQuery(ctx, &query) {
		return
	}

	if h.uc == nil {
		InternalError(ctx, "migration service not configured")
		return
	}

	dist, err := h.uc.StorageDistribution(ctx.Request.Context(), actorFromContext(ctx), uri.LibraryID, query.NodeID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	out := make([]migrationpg.ProviderDistribution, 0, len(dist))
	out = append(out, dist...)
	Success(ctx, gin.H{"byProvider": out})
}

func toTaskResponse(t domainmigration.Task) taskResponse {
	resp := taskResponse{
		ID:               t.ID,
		ActorID:          t.ActorID,
		LibraryID:        t.LibraryID,
		RootNodeID:       t.RootNodeID,
		TargetProvider:   t.TargetProvider,
		Status:           t.Status,
		TotalObjects:     t.TotalObjects,
		CompletedObjects: t.CompletedObjects,
		FailedObjects:    t.FailedObjects,
		SkippedObjects:   t.SkippedObjects,
		TotalBytes:       t.TotalBytes,
		TransferredBytes: t.TransferredBytes,
		CurrentObjectKey: t.CurrentObjectKey,
		ErrorMessage:     t.ErrorMessage,
		CreatedAt:        t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        t.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if t.StartedAt != nil {
		s := t.StartedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		resp.StartedAt = &s
	}
	if t.FinishedAt != nil {
		s := t.FinishedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		resp.FinishedAt = &s
	}
	return resp
}

func toTaskItemResponse(it domainmigration.TaskItem) taskItemResponse {
	resp := taskItemResponse{
		ID:                    it.ID,
		TaskID:                it.TaskID,
		StorageObjectID:       it.StorageObjectID,
		SourceProvider:        it.SourceProvider,
		SourceBucket:          it.SourceBucket,
		SourceKey:             it.SourceKey,
		TargetStorageObjectID: it.TargetStorageObjectID,
		TargetKey:             it.TargetKey,
		FileSize:              it.FileSize,
		Status:                it.Status,
		ErrorMessage:          it.ErrorMessage,
		CreatedAt:             it.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if it.StartedAt != nil {
		s := it.StartedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		resp.StartedAt = &s
	}
	if it.FinishedAt != nil {
		s := it.FinishedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		resp.FinishedAt = &s
	}
	return resp
}

func splitCommaList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
