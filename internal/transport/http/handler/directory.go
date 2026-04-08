package handler

import (
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

func (h *DirectoryHandler) Register(group *gin.RouterGroup) {
	group.POST("/upload", h.UploadFile)
	group.GET("/link", h.GetFileLink)
}

type fileLinkQuery struct {
	NodeID    uint64 `form:"node_id"`
	LibraryID uint64 `form:"library_id"`
	Expiry    int    `form:"expiry"`
}

func (h *DirectoryHandler) UploadFile(ctx *gin.Context) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		BadRequest(ctx, "file is required")
		return
	}

	parentID, ok := PostFormUint64(ctx, true, "parent_id", "parentId")
	if !ok {
		return
	}
	libraryID, ok := PostFormUint64(ctx, true, "library_id", "libraryId")
	if !ok {
		return
	}

	if h.directoryUseCase == nil {
		Success(ctx, map[string]any{
			"id":        0,
			"name":      fileHeader.Filename,
			"type":      "file",
			"parentId":  parentID,
			"libraryId": libraryID,
		})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		InternalError(ctx, "open upload file failed")
		return
	}
	defer file.Close()

	node, err := h.directoryUseCase.UploadAndCreateNode(ctx.Request.Context(), usecase.UploadFileCommand{
		Actor:       actorFromContext(ctx),
		LibraryID:   libraryID,
		ParentID:    parentID,
		FileName:    fileHeader.Filename,
		FileSize:    fileHeader.Size,
		ContentType: fileHeader.Header.Get("Content-Type"),
		Content:     file,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, node)
}

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
		Success(ctx, "")
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
