package handler

import (
	"github.com/gin-gonic/gin"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/usecase"
)

type nodeMetadataRequest struct {
	LibraryID    uint64          `json:"libraryId" binding:"required"`
	Mode         string          `json:"mode" binding:"omitempty,oneof=root children search node"`
	NodeID       uint64          `json:"nodeId"`
	ParentID     uint64          `json:"parentId"`
	AncestorID   uint64          `json:"ancestorId"`
	Keyword      string          `json:"keyword" binding:"max=512"`
	Names        []string        `json:"names" binding:"max=2,dive,max=512"`
	NodeType     domainnode.Type `json:"nodeType" binding:"omitempty,oneof=dir file"`
	TagIDs       []uint64        `json:"tagIds" binding:"max=50,dive,gt=0"`
	TagMatchMode string          `json:"tagMatchMode" binding:"omitempty,oneof=ANY ALL"`
	Cursor       string          `json:"cursor" binding:"max=256"`
	Limit        int             `json:"limit" binding:"omitempty,min=1,max=100"`
}

// BrowseMetadata 分页查询逻辑节点，不接触对象存储。
func (h *NodeHandler) BrowseMetadata(ctx *gin.Context) {
	var req nodeMetadataRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if h.nodeUseCase == nil {
		InternalError(ctx, "node service not configured")
		return
	}
	page, err := h.nodeUseCase.BrowseNodeMetadata(ctx.Request.Context(), usecase.BrowseNodeMetadataQuery{
		Actor: actorFromContext(ctx), Cursor: req.Cursor, Limit: req.Limit,
		Query: domainnode.MetadataQuery{LibraryID: req.LibraryID, Mode: req.Mode, ParentID: req.ParentID, NodeID: req.NodeID,
			AncestorID: req.AncestorID, Keyword: req.Keyword, Names: req.Names, NodeType: req.NodeType, TagIDs: req.TagIDs, TagMatchMode: req.TagMatchMode},
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, page)
}
