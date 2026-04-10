package handler

import "omniflow-go/internal/usecase"

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

	rootNodeID, err := h.nodeUseCase.GetLibraryRootNodeID(ctx.Request.Context(), actorFromContext(ctx), uri.LibraryID)
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

	nodes, err := h.nodeUseCase.GetAllDescendants(ctx.Request.Context(), actorFromContext(ctx), nodeID, libraryID)
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

	nodes, err := h.nodeUseCase.GetDirectChildren(ctx.Request.Context(), actorFromContext(ctx), nodeID, libraryID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, nodes)
}

// GetArchiveCards 分页获取归档页卡片（按内置类型过滤，并返回封面节点信息）。
func (h *NodeHandler) GetArchiveCards(ctx *gin.Context) {
	var uri nodeURI
	if !BindURI(ctx, &uri) {
		return
	}

	var query archiveCardsQuery
	if !BindQuery(ctx, &query) {
		return
	}
	if query.LibraryID == 0 {
		var ok bool
		query.LibraryID, ok = QueryUint64(ctx, true, "libraryId", "library_id")
		if !ok {
			return
		}
	}

	if h.nodeUseCase == nil {
		Success(ctx, usecase.ListArchiveCardsResult{
			Items:   []usecase.ArchiveCardItem{},
			Total:   0,
			Offset:  query.Offset,
			Limit:   query.Limit,
			HasMore: false,
		})
		return
	}

	result, err := h.nodeUseCase.ListArchiveCards(ctx.Request.Context(), usecase.ListArchiveCardsQuery{
		Actor:       actorFromContext(ctx),
		LibraryID:   query.LibraryID,
		NodeID:      uri.NodeID,
		BuiltInType: query.BuiltInType,
		Offset:      query.Offset,
		Limit:       query.Limit,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, result)
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

	nodes, err := h.nodeUseCase.GetAncestors(ctx.Request.Context(), actorFromContext(ctx), nodeID, libraryID)
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

	path, err := h.nodeUseCase.GetFullPath(ctx.Request.Context(), actorFromContext(ctx), nodeID, libraryID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, path)
}
