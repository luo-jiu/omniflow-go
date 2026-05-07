package handler

import (
	"net/http"
	"strings"

	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

// UploadHandler 暴露直传 MinIO 流程的 7 个端点：init / parts/sign / parts(GET) / renew / complete / abort。
// 鉴权由 actorFromContext 注入；usecase 层做 actor 与 session.actor 一致性校验，
// 不一致 / 不存在统一返回 404 防 uploadId 枚举，过期返回 410 Gone（参考 result.HandleUseCaseError 映射）。
type UploadHandler struct {
	uc *usecase.UploadSessionUseCase
}

func NewUploadHandler(uc *usecase.UploadSessionUseCase) *UploadHandler {
	return &UploadHandler{uc: uc}
}

type initUploadRequest struct {
	LibraryID       uint64 `json:"libraryId" binding:"required"`
	ParentID        uint64 `json:"parentId"`
	FileName        string `json:"fileName" binding:"required"`
	FileSize        int64  `json:"fileSize" binding:"required,min=1"`
	ContentType     string `json:"contentType"`
	StorageProvider string `json:"storageProvider"`
}

// PartNumbers 同时限制下界 1（防空请求）和上界 200（防恶意客户端一次签 10000 个膨胀响应；
// 前端/CLI 实际并发 4，200 足够留头）。元素值由 usecase 进一步校验 ≥1。
type signUploadPartsRequest struct {
	UploadID    string `json:"uploadId" binding:"required"`
	PartNumbers []int  `json:"partNumbers" binding:"required,min=1,max=200"`
}

type completedPartDTO struct {
	PartNumber int    `json:"partNumber" binding:"required,min=1"`
	ETag       string `json:"etag" binding:"required"`
}

type completeUploadRequest struct {
	UploadID       string             `json:"uploadId" binding:"required"`
	Parts          []completedPartDTO `json:"parts"`
	ConflictPolicy string             `json:"conflictPolicy"`
}

// Init 创建上传会话。
func (h *UploadHandler) Init(ctx *gin.Context) {
	var req initUploadRequest
	if !BindJSON(ctx, &req) {
		return
	}
	result, err := h.uc.Init(ctx.Request.Context(), usecase.InitUploadSessionCommand{
		Actor:           actorFromContext(ctx),
		LibraryID:       req.LibraryID,
		ParentID:        req.ParentID,
		FileName:        req.FileName,
		FileSize:        req.FileSize,
		ContentType:     req.ContentType,
		StorageProvider: req.StorageProvider,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, result)
}

// SignParts 颁发分片预签名 URL，并隐式续约 lease。
func (h *UploadHandler) SignParts(ctx *gin.Context) {
	var req signUploadPartsRequest
	if !BindJSON(ctx, &req) {
		return
	}
	result, err := h.uc.SignParts(ctx.Request.Context(), usecase.SignUploadPartsCommand{
		Actor:       actorFromContext(ctx),
		UploadID:    req.UploadID,
		PartNumbers: req.PartNumbers,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, result)
}

// ListParts 透传 MinIO ListParts 用于断点续传。
func (h *UploadHandler) ListParts(ctx *gin.Context) {
	uploadID := strings.TrimSpace(ctx.Query("uploadId"))
	if uploadID == "" {
		BadRequest(ctx, "uploadId is required")
		return
	}
	parts, err := h.uc.ListParts(ctx.Request.Context(), actorFromContext(ctx), uploadID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, gin.H{"parts": parts})
}

// Renew 心跳兜底续约 lease。
func (h *UploadHandler) Renew(ctx *gin.Context) {
	uploadID := strings.TrimSpace(ctx.Param("uploadId"))
	if uploadID == "" {
		BadRequest(ctx, "uploadId is required")
		return
	}
	expiresAt, err := h.uc.Renew(ctx.Request.Context(), actorFromContext(ctx), uploadID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, gin.H{"expiresAt": expiresAt})
}

// Complete 提交分片清单（multipart）或校验对象（single）→ 创建 node。
func (h *UploadHandler) Complete(ctx *gin.Context) {
	var req completeUploadRequest
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
	node, err := h.uc.Complete(ctx.Request.Context(), usecase.CompleteUploadSessionCommand{
		Actor:          actorFromContext(ctx),
		UploadID:       req.UploadID,
		Parts:          parts,
		ConflictPolicy: usecase.NodeNameConflictPolicy(req.ConflictPolicy),
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, node)
}

// Abort 取消上传，回收 MinIO + 删 session。
func (h *UploadHandler) Abort(ctx *gin.Context) {
	uploadID := strings.TrimSpace(ctx.Param("uploadId"))
	if uploadID == "" {
		BadRequest(ctx, "uploadId is required")
		return
	}
	if err := h.uc.Abort(ctx.Request.Context(), actorFromContext(ctx), uploadID); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
