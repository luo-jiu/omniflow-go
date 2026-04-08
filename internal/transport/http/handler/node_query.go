package handler

import "github.com/gin-gonic/gin"

// GetLibraryRootNodeID 获取资料库根节点 ID；当根节点缺失时由后端自动补齐。
func (h *NodeHandler) GetLibraryRootNodeID(ctx *gin.Context) {
	var uri libraryRootURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.nodeUseCase == nil {
		Success(ctx, uint64(0))
		return
	}

	rootNodeID, err := h.nodeUseCase.GetLibraryRootNodeID(ctx.Request.Context(), uri.LibraryID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, rootNodeID)
}

// GetAllDescendants 获取当前节点及其完整子树。
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

// GetDirectChildren 获取当前节点的直接子节点。
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

// GetAncestors 获取从根到当前节点的祖先链。
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

// GetFullPath 获取节点的完整路径字符串。
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
