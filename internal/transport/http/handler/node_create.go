package handler

import (
	"strings"

	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

// CreateNode handles node creation (directory or file metadata node).
func (h *NodeHandler) CreateNode(ctx *gin.Context) {
	var req createNodeRequest
	if !BindJSON(ctx, &req) {
		return
	}

	if h.nodeUseCase == nil {
		Success(ctx, map[string]any{
			"id":        0,
			"name":      req.Name,
			"type":      req.Type,
			"parentId":  req.ParentID,
			"libraryId": req.LibraryID,
			"ext":       req.Ext,
			"mimeType":  req.MIMEType,
			"fileSize":  req.FileSize,
		})
		return
	}

	nodeType := domainnode.TypeDirectory
	if req.Type == 1 {
		nodeType = domainnode.TypeFile
	}

	created, err := h.nodeUseCase.Create(ctx.Request.Context(), usecase.CreateNodeCommand{
		Actor:      actorFromContext(ctx),
		Name:       strings.TrimSpace(req.Name),
		Type:       nodeType,
		ParentID:   req.ParentID,
		LibraryID:  req.LibraryID,
		Ext:        strings.TrimSpace(req.Ext),
		MIMEType:   strings.TrimSpace(req.MIMEType),
		FileSize:   req.FileSize,
		StorageKey: strings.TrimSpace(req.StorageKey),
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, created)
}
