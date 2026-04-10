package handler

import (
	"strings"

	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

// SearchNodes 节点搜索（名称 + tagIds 组合）。
func (h *NodeHandler) SearchNodes(ctx *gin.Context) {
	var req searchNodesRequest
	if !BindJSON(ctx, &req) {
		return
	}

	if h.nodeUseCase == nil {
		InternalError(ctx, "node service not configured")
		return
	}

	nodes, err := h.nodeUseCase.SearchNodes(ctx.Request.Context(), usecase.SearchNodesQuery{
		Actor:        actorFromContext(ctx),
		LibraryID:    req.LibraryID,
		Keyword:      strings.TrimSpace(req.Keyword),
		TagIDs:       req.TagIDs,
		TagMatchMode: req.TagMatchMode,
		Limit:        req.Limit,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, nodes)
}
