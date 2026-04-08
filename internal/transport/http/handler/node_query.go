package handler

import "github.com/gin-gonic/gin"

// GetAllDescendants returns current node and full subtree.
func (h *NodeHandler) GetAllDescendants(ctx *gin.Context) {
	nodeID, libraryID, ok := h.parseNodeScope(ctx)
	if !ok {
		return
	}

	if h.nodeUseCase == nil {
		Success(ctx, []any{})
		return
	}

	nodes, err := h.nodeUseCase.GetAllDescendants(ctx.Request.Context(), nodeID, libraryID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, nodes)
}

// GetDirectChildren returns direct child nodes only.
func (h *NodeHandler) GetDirectChildren(ctx *gin.Context) {
	nodeID, libraryID, ok := h.parseNodeScope(ctx)
	if !ok {
		return
	}

	if h.nodeUseCase == nil {
		Success(ctx, []any{})
		return
	}

	nodes, err := h.nodeUseCase.GetDirectChildren(ctx.Request.Context(), nodeID, libraryID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, nodes)
}

// GetAncestors returns ancestor chain from root to current node.
func (h *NodeHandler) GetAncestors(ctx *gin.Context) {
	nodeID, libraryID, ok := h.parseNodeScope(ctx)
	if !ok {
		return
	}

	if h.nodeUseCase == nil {
		Success(ctx, []any{})
		return
	}

	nodes, err := h.nodeUseCase.GetAncestors(ctx.Request.Context(), nodeID, libraryID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, nodes)
}

// GetFullPath returns slash-joined full path for a node.
func (h *NodeHandler) GetFullPath(ctx *gin.Context) {
	nodeID, libraryID, ok := h.parseNodeScope(ctx)
	if !ok {
		return
	}

	if h.nodeUseCase == nil {
		Success(ctx, "")
		return
	}

	path, err := h.nodeUseCase.GetFullPath(ctx.Request.Context(), nodeID, libraryID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, path)
}
