package handler

import (
	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	fileUseCase *usecase.FileUseCase
}

func NewFileHandler(fileUseCase *usecase.FileUseCase) *FileHandler {
	return &FileHandler{fileUseCase: fileUseCase}
}

// UploadFile 上传对象文件并返回访问链接。
func (h *FileHandler) UploadFile(ctx *gin.Context) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		BadRequest(ctx, "file is required")
		return
	}
	if h.fileUseCase == nil {
		InternalError(ctx, "file service not configured")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		InternalError(ctx, "open upload file failed")
		return
	}
	defer file.Close()

	url, err := h.fileUseCase.UploadAndGetLink(ctx.Request.Context(), usecase.UploadObjectCommand{
		Path:        PostFormString(ctx, "path"),
		FileName:    fileHeader.Filename,
		FileSize:    fileHeader.Size,
		ContentType: fileHeader.Header.Get("Content-Type"),
		Content:     file,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, url)
}

// GetFileLink 按路径和文件名获取对象访问链接。
func (h *FileHandler) GetFileLink(ctx *gin.Context) {
	if h.fileUseCase == nil {
		InternalError(ctx, "file service not configured")
		return
	}

	fileName := QueryString(ctx, "file_name", "fileName")
	if !RequireNonEmpty(ctx, fileName, "file_name") {
		return
	}

	expiry, ok := QueryInt(ctx, 60, false, "expiry")
	if !ok {
		return
	}

	url, err := h.fileUseCase.GetFileLink(ctx.Request.Context(), usecase.GetObjectLinkQuery{
		Path:          QueryString(ctx, "path"),
		FileName:      fileName,
		ExpiryMinutes: expiry,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, url)
}
