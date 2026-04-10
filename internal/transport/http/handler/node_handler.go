package handler

import (
	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

type NodeHandler struct {
	nodeUseCase *usecase.NodeUseCase
}

func NewNodeHandler(nodeUseCase *usecase.NodeUseCase) *NodeHandler {
	return &NodeHandler{nodeUseCase: nodeUseCase}
}

type nodeURI struct {
	NodeID uint64 `uri:"nodeId" binding:"required"`
}

type nodeLibraryQuery struct {
	LibraryID uint64 `form:"libraryId"`
}

type archiveCardsQuery struct {
	LibraryID   uint64 `form:"libraryId"`
	BuiltInType string `form:"builtInType"`
	Offset      int    `form:"offset"`
	Limit       int    `form:"limit"`
}

type deleteNodeURI struct {
	NodeID    uint64 `uri:"nodeId" binding:"required"`
	LibraryID uint64 `uri:"libraryId" binding:"required"`
}

type libraryRootURI struct {
	LibraryID uint64 `uri:"libraryId" binding:"required"`
}

type createNodeRequest struct {
	Name       string `json:"name" binding:"required"`
	Ext        string `json:"ext"`
	MIMEType   string `json:"mimeType"`
	FileSize   int64  `json:"fileSize"`
	StorageKey string `json:"storageKey"`
	Type       int    `json:"type"`
	ParentID   uint64 `json:"parentId"`
	LibraryID  uint64 `json:"libraryId" binding:"required"`
}

type searchNodesRequest struct {
	LibraryID    uint64   `json:"libraryId" binding:"required"`
	Keyword      string   `json:"keyword"`
	TagIDs       []uint64 `json:"tagIds"`
	TagMatchMode string   `json:"tagMatchMode"`
	Limit        int      `json:"limit"`
}

type updateNodeRequest struct {
	BuiltInType *string `json:"builtInType"`
	ArchiveMode *int    `json:"archiveMode"`
	ViewMeta    *string `json:"viewMeta"`
}

type renameNodeRequest struct {
	Name string  `json:"name" binding:"required"`
	Ext  *string `json:"ext"`
}

type moveNodeRequest struct {
	Name         string `json:"name"`
	NodeID       uint64 `json:"nodeId"`
	NewParentID  uint64 `json:"newParentId" binding:"required"`
	BeforeNodeID uint64 `json:"beforeNodeId"`
	LibraryID    uint64 `json:"libraryId" binding:"required"`
}

// parseNodeScope 从 URI 解析节点 ID，并从查询参数解析资料库 ID。
func (h *NodeHandler) parseNodeScope(ctx *gin.Context) (uint64, uint64, bool) {
	var uri nodeURI
	if !BindURI(ctx, &uri) {
		return 0, 0, false
	}

	var query nodeLibraryQuery
	if !BindQuery(ctx, &query) {
		return 0, 0, false
	}

	if query.LibraryID == 0 {
		var ok bool
		query.LibraryID, ok = QueryUint64(ctx, true, "libraryId", "library_id")
		if !ok {
			return 0, 0, false
		}
	}

	return uri.NodeID, query.LibraryID, true
}
