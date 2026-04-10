package handler

import "github.com/gin-gonic/gin"

// GetNodeDetail 按节点 ID 查询节点详情。
func (h *NodeHandler) GetNodeDetail(ctx *gin.Context) {
	var uri nodeURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.nodeUseCase == nil {
		InternalError(ctx, "node service not configured")
		return
	}

	node, err := h.nodeUseCase.GetNodeDetail(ctx.Request.Context(), actorFromContext(ctx), uri.NodeID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, node)
}
