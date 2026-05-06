package handler

import (
	"errors"
	"net/http"
	"strings"

	"omniflow-go/internal/uploadprogress"

	"github.com/gin-gonic/gin"
)

// UploadProgressHandler 暴露后端 → MinIO 阶段的真进度查询。
// 该 handler 跨整传与分片：客户端持有 uploadID，服务端按 actor 校验。
type UploadProgressHandler struct {
	tracker uploadprogress.Tracker
}

func NewUploadProgressHandler(tracker uploadprogress.Tracker) *UploadProgressHandler {
	return &UploadProgressHandler{tracker: tracker}
}

type uploadProgressResponse struct {
	UploadID      string  `json:"uploadId"`
	TotalBytes    int64   `json:"totalBytes"`
	UploadedBytes int64   `json:"uploadedBytes"`
	Percentage    float64 `json:"percentage"`
	State         string  `json:"state"`
}

// GetProgress 返回指定 uploadID 的上传字节进度。
// uploadID 不存在或 actor 不匹配时统一返回 404，避免 uploadID 枚举。
func (h *UploadProgressHandler) GetProgress(ctx *gin.Context) {
	uploadID := strings.TrimSpace(ctx.Param("uploadId"))
	if uploadID == "" {
		BadRequest(ctx, "uploadId is required")
		return
	}

	if h.tracker == nil {
		InternalError(ctx, "upload progress tracker not configured")
		return
	}

	act := actorFromContext(ctx)

	progress, err := h.tracker.Get(ctx.Request.Context(), uploadID, act.ID)
	if err != nil {
		if errors.Is(err, uploadprogress.ErrNotFound) {
			respond(ctx, http.StatusNotFound, ClientErrorCode, "upload progress not found", nil)
			return
		}
		_ = ctx.Error(err)
		InternalError(ctx, err.Error())
		return
	}

	Success(ctx, uploadProgressResponse{
		UploadID:      progress.UploadID,
		TotalBytes:    progress.TotalBytes,
		UploadedBytes: progress.UploadedBytes,
		Percentage:    progress.Percentage,
		State:         string(progress.State),
	})
}
