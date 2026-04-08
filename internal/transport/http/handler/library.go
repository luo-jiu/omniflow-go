package handler

import (
	"strings"

	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

type LibraryHandler struct {
	libraryUseCase *usecase.LibraryUseCase
}

func NewLibraryHandler(libraryUseCase *usecase.LibraryUseCase) *LibraryHandler {
	return &LibraryHandler{libraryUseCase: libraryUseCase}
}

type scrollLibrariesQuery struct {
	LastID uint64 `form:"lastId"`
	Size   int    `form:"size"`
}

type libraryIDURI struct {
	ID uint64 `uri:"id" binding:"required"`
}

type createLibraryRequest struct {
	Name string `json:"name" binding:"required"`
}

type updateLibraryRequest struct {
	Name string `json:"name" binding:"required"`
}

// Scroll 分页查询当前用户的资料库列表。
func (h *LibraryHandler) Scroll(ctx *gin.Context) {
	var query scrollLibrariesQuery
	if !BindQuery(ctx, &query) {
		return
	}
	if query.Size <= 0 {
		query.Size = 10
	}

	if h.libraryUseCase == nil {
		Success(ctx, []any{})
		return
	}

	result, err := h.libraryUseCase.Scroll(ctx.Request.Context(), usecase.ListLibrariesQuery{
		Actor:  actorFromContext(ctx),
		LastID: query.LastID,
		Size:   query.Size,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, result)
}

// Create 创建资料库。
func (h *LibraryHandler) Create(ctx *gin.Context) {
	var req createLibraryRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if !RequireNonEmpty(ctx, req.Name, "name") {
		return
	}

	if h.libraryUseCase == nil {
		SuccessNoData(ctx)
		return
	}

	created, err := h.libraryUseCase.Create(ctx.Request.Context(), usecase.CreateLibraryCommand{
		Actor: actorFromContext(ctx),
		Name:  strings.TrimSpace(req.Name),
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, created)
}

// Update 按 ID 更新资料库信息。
func (h *LibraryHandler) Update(ctx *gin.Context) {
	var uri libraryIDURI
	if !BindURI(ctx, &uri) {
		return
	}

	var req updateLibraryRequest
	if !BindJSON(ctx, &req) {
		return
	}

	if h.libraryUseCase == nil {
		SuccessNoData(ctx)
		return
	}

	if err := h.libraryUseCase.Update(ctx.Request.Context(), uri.ID, usecase.UpdateLibraryCommand{
		Actor: actorFromContext(ctx),
		Name:  strings.TrimSpace(req.Name),
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// Delete 按 ID 软删除资料库。
func (h *LibraryHandler) Delete(ctx *gin.Context) {
	var uri libraryIDURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.libraryUseCase == nil {
		SuccessNoData(ctx)
		return
	}

	if err := h.libraryUseCase.Delete(ctx.Request.Context(), usecase.DeleteLibraryCommand{
		Actor: actorFromContext(ctx),
		ID:    uri.ID,
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}
