package handler

import (
	"strings"
	"time"

	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

type DirectoryHandler struct {
	directoryUseCase *usecase.DirectoryUseCase
}

func NewDirectoryHandler(directoryUseCase *usecase.DirectoryUseCase) *DirectoryHandler {
	return &DirectoryHandler{directoryUseCase: directoryUseCase}
}

type fileLinkQuery struct {
	NodeID    uint64 `form:"node_id"`
	LibraryID uint64 `form:"library_id"`
	Expiry    int    `form:"expiry"`
}

type batchFileLinkRequest struct {
	LibraryID uint64   `json:"libraryId" binding:"required"`
	NodeIDs   []uint64 `json:"nodeIds" binding:"required"`
	Expiry    int      `json:"expiry"`
}

type updateFileContentRequest struct {
	LibraryID       uint64  `json:"libraryId" binding:"required"`
	Content         *string `json:"content" binding:"required"`
	ContentType     string  `json:"contentType"`
	StorageProvider string  `json:"storageProvider"`
}

// GetFileLink 获取目录文件节点的预签名链接。
func (h *DirectoryHandler) GetFileLink(ctx *gin.Context) {
	var query fileLinkQuery
	if !BindQuery(ctx, &query) {
		return
	}

	if query.NodeID == 0 {
		var ok bool
		query.NodeID, ok = QueryUint64(ctx, true, "node_id", "nodeId")
		if !ok {
			return
		}
	}
	if query.LibraryID == 0 {
		var ok bool
		query.LibraryID, ok = QueryUint64(ctx, true, "library_id", "libraryId")
		if !ok {
			return
		}
	}

	expiry, ok := QueryInt(ctx, 60, false, "expiry")
	if !ok {
		return
	}
	query.Expiry = expiry
	if query.Expiry <= 0 {
		query.Expiry = 60
	}

	if h.directoryUseCase == nil {
		InternalError(ctx, "directory service not configured")
		return
	}

	url, err := h.directoryUseCase.GetPresignedURL(ctx.Request.Context(), usecase.GetFileLinkQuery{
		Actor:     actorFromContext(ctx),
		LibraryID: query.LibraryID,
		NodeID:    query.NodeID,
		Expiry:    time.Duration(query.Expiry) * time.Minute,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, url)
}

// BatchGetFileLinks 批量获取目录文件节点的预签名链接。
func (h *DirectoryHandler) BatchGetFileLinks(ctx *gin.Context) {
	var req batchFileLinkRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if req.LibraryID == 0 {
		BadRequest(ctx, "libraryId is required")
		return
	}
	if len(req.NodeIDs) == 0 {
		Success(ctx, []usecase.BatchFileLinkItem{})
		return
	}
	if req.Expiry <= 0 {
		req.Expiry = 60
	}

	if h.directoryUseCase == nil {
		InternalError(ctx, "directory service not configured")
		return
	}

	items, err := h.directoryUseCase.BatchGetPresignedURLs(ctx.Request.Context(), usecase.BatchGetFileLinksQuery{
		Actor:     actorFromContext(ctx),
		LibraryID: req.LibraryID,
		NodeIDs:   req.NodeIDs,
		Expiry:    time.Duration(req.Expiry) * time.Minute,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, items)
}

// UpdateFileContent 按节点 ID 更新文件内容，保留节点身份不变。
func (h *DirectoryHandler) UpdateFileContent(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var uri nodeURI
	if !BindURI(ctx, &uri) {
		return
	}

	var req updateFileContentRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if req.Content == nil {
		BadRequest(ctx, "content is required")
		return
	}

	if h.directoryUseCase == nil {
		InternalError(ctx, "directory service not configured")
		return
	}

	contentBytes := []byte(*req.Content)
	node, err := h.directoryUseCase.UpdateFileContent(ctx.Request.Context(), usecase.UpdateFileContentCommand{
		Actor:           actorFromContext(ctx),
		LibraryID:       req.LibraryID,
		NodeID:          uri.NodeID,
		FileSize:        int64(len(contentBytes)),
		ContentType:     req.ContentType,
		StorageProvider: strings.TrimSpace(req.StorageProvider),
		Content:         strings.NewReader(*req.Content),
		DryRun:          dryRun,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessWithDryRun(ctx, dryRun, node)
}
