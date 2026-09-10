package handler

import (
	"github.com/gin-gonic/gin"
	"omniflow-go/internal/usecase"
)

// CopyNode 复制同库节点，目录必须显式允许递归。
func (h *NodeHandler) CopyNode(ctx *gin.Context) {
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
		LibraryID      uint64 `json:"libraryId" binding:"required"`
		ParentID       uint64 `json:"parentId" binding:"required"`
		Name           string `json:"name"`
		Recursive      bool   `json:"recursive"`
		ConflictPolicy string `json:"conflictPolicy" binding:"omitempty,oneof=error auto_rename"`
	}
	if !BindJSON(ctx, &req) {
		return
	}
	if h.nodeUseCase == nil {
		InternalError(ctx, "node service not configured")
		return
	}
	result, err := h.nodeUseCase.Copy(ctx.Request.Context(), usecase.CopyNodeCommand{
		Actor: actorFromContext(ctx), NodeID: uri.NodeID, LibraryID: req.LibraryID,
		ParentID: req.ParentID, Name: req.Name, Recursive: req.Recursive,
		ConflictPolicy: usecase.NodeNameConflictPolicy(req.ConflictPolicy), DryRun: dryRun,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, result)
}
