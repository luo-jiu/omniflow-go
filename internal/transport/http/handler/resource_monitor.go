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
