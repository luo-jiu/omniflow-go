package handler

import (
	"net/http"
	"strconv"

	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

// MultipartUploadHandler 处理分片上传相关的 HTTP 请求。
type MultipartUploadHandler struct {
	uc *usecase.MultipartUploadUseCase
}

// NewMultipartUploadHandler 创建分片上传 Handler。
func NewMultipartUploadHandler(uc *usecase.MultipartUploadUseCase) *MultipartUploadHandler {
	return &MultipartUploadHandler{uc: uc}
}

type initiateMultipartRequest struct {
	LibraryID      uint64 `json:"libraryId" binding:"required"`
	ParentID       uint64 `json:"parentId"`
	FileName       string `json:"fileName" binding:"required"`
	FileSize       int64  `json:"fileSize" binding:"required,min=1"`
	ContentType    string `json:"contentType"`
	ConflictPolicy string `json:"conflictPolicy"`
}

type completeMultipartRequest struct {
	Parts []completedPartDTO `json:"parts" binding:"required,min=1"`
}

type completedPartDTO struct {
	PartNumber int    `json:"partNumber" binding:"required,min=1"`
	ETag       string `json:"etag" binding:"required"`
}

// InitiateUpload 发起分片上传。
func (h *MultipartUploadHandler) InitiateUpload(ctx *gin.Context) {
	var req initiateMultipartRequest
	if !BindJSON(ctx, &req) {
		return
	}

	result, err := h.uc.Initiate(ctx.Request.Context(), usecase.InitiateMultipartUploadCommand{
		Actor:          actorFromContext(ctx),
		LibraryID:      req.LibraryID,
		ParentID:       req.ParentID,
		FileName:       req.FileName,
		FileSize:       req.FileSize,
		ContentType:    req.ContentType,
		ConflictPolicy: usecase.NodeNameConflictPolicy(req.ConflictPolicy),
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, result)
}

// UploadPart 上传单个分片。
func (h *MultipartUploadHandler) UploadPart(ctx *gin.Context) {
	uploadID := ctx.Param("uploadId")
	if uploadID == "" {
		BadRequest(ctx, "uploadId is required")
		return
	}

	partNumberStr := ctx.Param("partNumber")
	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil || partNumber < 1 {
		BadRequest(ctx, "partNumber must be a positive integer")
		return
	}

	size := ctx.Request.ContentLength
	if size <= 0 {
		BadRequest(ctx, "Content-Length is required and must be > 0")
		return
	}

	result, err := h.uc.UploadPart(ctx.Request.Context(), usecase.UploadPartCommand{
		Actor:      actorFromContext(ctx),
		UploadID:   uploadID,
		PartNumber: partNumber,
		Body:       ctx.Request.Body,
		Size:       size,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, result)
}

// CompleteUpload 合并分片并创建文件节点。
func (h *MultipartUploadHandler) CompleteUpload(ctx *gin.Context) {
	uploadID := ctx.Param("uploadId")
	if uploadID == "" {
		BadRequest(ctx, "uploadId is required")
		return
	}

	var req completeMultipartRequest
	if !BindJSON(ctx, &req) {
		return
	}

	parts := make([]usecase.CompletedPart, len(req.Parts))
	for i, p := range req.Parts {
		parts[i] = usecase.CompletedPart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		}
	}

	node, err := h.uc.Complete(ctx.Request.Context(), usecase.CompleteMultipartUploadCommand{
		Actor:    actorFromContext(ctx),
		UploadID: uploadID,
		Parts:    parts,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, node)
}

// AbortUpload 取消分片上传。
func (h *MultipartUploadHandler) AbortUpload(ctx *gin.Context) {
	uploadID := ctx.Param("uploadId")
	if uploadID == "" {
		BadRequest(ctx, "uploadId is required")
		return
	}

	err := h.uc.Abort(ctx.Request.Context(), actorFromContext(ctx), uploadID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ListParts 列出已上传的分片。
func (h *MultipartUploadHandler) ListParts(ctx *gin.Context) {
	uploadID := ctx.Param("uploadId")
	if uploadID == "" {
		BadRequest(ctx, "uploadId is required")
		return
	}

	parts, err := h.uc.ListParts(ctx.Request.Context(), actorFromContext(ctx), uploadID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, map[string]any{"parts": parts})
}
