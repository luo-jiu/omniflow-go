package handler

import (
	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

// GetRecycleBinItems 获取资料库回收站顶层条目。
func (h *NodeHandler) GetRecycleBinItems(ctx *gin.Context) {
	var uri libraryRootURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.nodeUseCase == nil {
		Success(ctx, []any{})
		return
	}

	items, err := h.nodeUseCase.GetRecycleBinItems(ctx.Request.Context(), uri.LibraryID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, items)
}

// RestoreNodeAndChildren 从回收站恢复节点及其后代。
func (h *NodeHandler) RestoreNodeAndChildren(ctx *gin.Context) {
	var uri deleteNodeURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.nodeUseCase == nil {
		Success(ctx, true)
		return
	}

	ok, err := h.nodeUseCase.RestoreNodeAndChildren(ctx.Request.Context(), usecase.RestoreNodeTreeCommand{
		Actor:     actorFromContext(ctx),
		LibraryID: uri.LibraryID,
		NodeID:    uri.NodeID,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, ok)
}

// HardDeleteNodeAndChildren 彻底删除回收站中的节点及其后代。
func (h *NodeHandler) HardDeleteNodeAndChildren(ctx *gin.Context) {
	var uri deleteNodeURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.nodeUseCase == nil {
		Success(ctx, true)
		return
	}

	ok, err := h.nodeUseCase.HardDeleteNodeAndChildren(ctx.Request.Context(), usecase.HardDeleteNodeTreeCommand{
		Actor:     actorFromContext(ctx),
		LibraryID: uri.LibraryID,
		NodeID:    uri.NodeID,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, ok)
}
