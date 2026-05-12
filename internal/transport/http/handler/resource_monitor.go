package handler

import (
	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

// ResourceMonitorHandler 处理资源监测控制台请求。
type ResourceMonitorHandler struct {
	uc *usecase.ResourceMonitorUseCase
}

// NewResourceMonitorHandler 创建资源监测 HTTP handler。
func NewResourceMonitorHandler(uc *usecase.ResourceMonitorUseCase) *ResourceMonitorHandler {
	return &ResourceMonitorHandler{uc: uc}
}

// Snapshot 返回当前用户可见资料库范围的资源监测快照。
func (h *ResourceMonitorHandler) Snapshot(ctx *gin.Context) {
	if h.uc == nil {
		InternalError(ctx, "resource monitor service not configured")
		return
	}
	libraryID, ok := QueryUint64(ctx, false, "libraryId")
	if !ok {
		return
	}
	snapshot, err := h.uc.Snapshot(
		ctx.Request.Context(),
		actorFromContext(ctx),
		usecase.ResourceMonitorSnapshotOptions{LibraryID: libraryID},
	)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, snapshot)
}

// Distribution 返回当前用户可见资料库范围的资源分布快照。
func (h *ResourceMonitorHandler) Distribution(ctx *gin.Context) {
	if h.uc == nil {
		InternalError(ctx, "resource monitor service not configured")
		return
	}
	libraryID, ok := QueryUint64(ctx, false, "libraryId")
	if !ok {
		return
	}
	snapshot, err := h.uc.Distribution(
		ctx.Request.Context(),
		actorFromContext(ctx),
		usecase.ResourceMonitorSnapshotOptions{LibraryID: libraryID},
	)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, snapshot)
}

// Probes 返回当前资源监测探针结果。
func (h *ResourceMonitorHandler) Probes(ctx *gin.Context) {
	if h.uc == nil {
		InternalError(ctx, "resource monitor service not configured")
		return
	}
	snapshot, err := h.uc.Probes(ctx.Request.Context(), actorFromContext(ctx))
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, snapshot)
}

// Breakdown 返回当前用户可见资料库范围的资源细分仪表盘。
func (h *ResourceMonitorHandler) Breakdown(ctx *gin.Context) {
	if h.uc == nil {
		InternalError(ctx, "resource monitor service not configured")
		return
	}
	libraryID, ok := QueryUint64(ctx, false, "libraryId")
	if !ok {
		return
	}
	breakdown, err := h.uc.Breakdown(
		ctx.Request.Context(),
		actorFromContext(ctx),
		usecase.ResourceMonitorSnapshotOptions{LibraryID: libraryID},
	)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, breakdown)
}

// Dashboard 返回当前用户可见资料库范围的 V2 资源统计仪表盘。
func (h *ResourceMonitorHandler) Dashboard(ctx *gin.Context) {
	if h.uc == nil {
		InternalError(ctx, "resource monitor service not configured")
		return
	}
	libraryID, ok := QueryUint64(ctx, false, "libraryId")
	if !ok {
		return
	}
	dashboard, err := h.uc.Dashboard(
		ctx.Request.Context(),
		actorFromContext(ctx),
		usecase.ResourceMonitorSnapshotOptions{LibraryID: libraryID},
	)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, dashboard)
}

// CaptureSample 显式写入一条资源监测历史采样。
func (h *ResourceMonitorHandler) CaptureSample(ctx *gin.Context) {
	libraryID, ok := QueryUint64(ctx, false, "libraryId")
	if !ok {
		return
	}
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	if h.uc == nil {
		InternalError(ctx, "resource monitor service not configured")
		return
	}
	sample, err := h.uc.CaptureSample(
		ctx.Request.Context(),
		actorFromContext(ctx),
		usecase.ResourceMonitorSnapshotOptions{LibraryID: libraryID, DryRun: dryRun},
	)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, sample)
}
