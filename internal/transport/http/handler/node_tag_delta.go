package handler

import (
	"github.com/gin-gonic/gin"
	"omniflow-go/internal/usecase"
)

// UpdateNodeTags 原子增删当前资料库节点标签。
func (h *NodeHandler) UpdateNodeTags(ctx *gin.Context) {
	var uri nodeURI
	if !BindURI(ctx, &uri) {
		return
	}
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)
	var req struct {
		LibraryID    uint64   `json:"libraryId" binding:"required"`
		AddTagIDs    []uint64 `json:"addTagIds" binding:"max=100,dive,gt=0"`
		RemoveTagIDs []uint64 `json:"removeTagIds" binding:"max=100,dive,gt=0"`
	}
	if !BindJSON(ctx, &req) {
		return
	}
	if h.nodeUseCase == nil {
		InternalError(ctx, "node service not configured")
		return
	}
	result, err := h.nodeUseCase.UpdateNodeTags(ctx.Request.Context(), usecase.NodeTagDeltaCommand{Actor: actorFromContext(ctx),
		NodeID: uri.NodeID, LibraryID: req.LibraryID, AddTagIDs: req.AddTagIDs, RemoveTagIDs: req.RemoveTagIDs, DryRun: dryRun})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessWithDryRun(ctx, dryRun, result)
}

// ReadNodeTags 返回正式节点标签关系。
func (h *NodeHandler) ReadNodeTags(ctx *gin.Context) {
	var uri nodeURI
	if !BindURI(ctx, &uri) {
		return
	}
	var req struct {
		LibraryID uint64 `form:"libraryId" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		BadRequest(ctx, err.Error())
		return
	}
	if h.nodeUseCase == nil {
		InternalError(ctx, "node service not configured")
		return
	}
	result, err := h.nodeUseCase.ReadNodeTags(ctx.Request.Context(), actorFromContext(ctx), uri.NodeID, req.LibraryID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, result)
}
