package handler

import (
	"time"

	domainnode "omniflow-go/internal/domain/node"

	"github.com/gin-gonic/gin"
)

type nodeDetailResponse struct {
	domainnode.Node
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func newNodeDetailResponse(node domainnode.Node) nodeDetailResponse {
	return nodeDetailResponse{
		Node:      node,
		CreatedAt: node.CreatedAt,
		UpdatedAt: node.UpdatedAt,
	}
}

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
	Success(ctx, newNodeDetailResponse(node))
}
