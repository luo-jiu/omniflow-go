package handler

import (
	"strings"

	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

type TagHandler struct {
	tagUseCase *usecase.TagUseCase
}

func NewTagHandler(tagUseCase *usecase.TagUseCase) *TagHandler {
	return &TagHandler{tagUseCase: tagUseCase}
}

// GetSearchTypes 返回前端可用的搜索类型标签。
func (h *TagHandler) GetSearchTypes(ctx *gin.Context) {
	if h.tagUseCase == nil {
		Success(ctx, "PostgreSQL")
		return
	}
	Success(ctx, h.tagUseCase.SearchType())
}

type listTagsQuery struct {
	Type string `form:"type"`
}

type tagIDURI struct {
	TagID uint64 `uri:"tagId" binding:"required"`
}

type tagCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Type        string `json:"type"`
	TargetKey   string `json:"targetKey"`
	Color       string `json:"color"`
	TextColor   string `json:"textColor"`
	SortOrder   *int   `json:"sortOrder"`
	Enabled     *int   `json:"enabled"`
	Description string `json:"description"`
}

type tagUpdateRequest struct {
	Name        string `json:"name" binding:"required"`
	Type        string `json:"type"`
	TargetKey   string `json:"targetKey"`
	Color       string `json:"color"`
	TextColor   string `json:"textColor"`
	SortOrder   *int   `json:"sortOrder"`
	Enabled     *int   `json:"enabled"`
	Description string `json:"description"`
}

// ListTags 查询标签（支持按 type 过滤）。
func (h *TagHandler) ListTags(ctx *gin.Context) {
	var query listTagsQuery
	if !BindQuery(ctx, &query) {
		return
	}

	if h.tagUseCase == nil {
		Success(ctx, []any{})
		return
	}

	tags, err := h.tagUseCase.List(ctx.Request.Context(), usecase.ListTagsQuery{
		Actor: actorFromContext(ctx),
		Type:  query.Type,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, tags)
}

// CreateTag 新建标签。
func (h *TagHandler) CreateTag(ctx *gin.Context) {
	var req tagCreateRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if !RequireNonEmpty(ctx, req.Name, "name") {
		return
	}

	if h.tagUseCase == nil {
		InternalError(ctx, "tag service not configured")
		return
	}

	tag, err := h.tagUseCase.Create(ctx.Request.Context(), usecase.CreateTagCommand{
		Actor:       actorFromContext(ctx),
		Name:        strings.TrimSpace(req.Name),
		Type:        strings.TrimSpace(req.Type),
		TargetKey:   strings.TrimSpace(req.TargetKey),
		Color:       strings.TrimSpace(req.Color),
		TextColor:   strings.TrimSpace(req.TextColor),
		SortOrder:   req.SortOrder,
		Enabled:     req.Enabled,
		Description: strings.TrimSpace(req.Description),
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, tag)
}

// UpdateTag 修改标签。
func (h *TagHandler) UpdateTag(ctx *gin.Context) {
	var uri tagIDURI
	if !BindURI(ctx, &uri) {
		return
	}

	var req tagUpdateRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if !RequireNonEmpty(ctx, req.Name, "name") {
		return
	}

	if h.tagUseCase == nil {
		InternalError(ctx, "tag service not configured")
		return
	}

	tag, err := h.tagUseCase.Update(ctx.Request.Context(), uri.TagID, usecase.UpdateTagCommand{
		Actor:       actorFromContext(ctx),
		Name:        strings.TrimSpace(req.Name),
		Type:        strings.TrimSpace(req.Type),
		TargetKey:   strings.TrimSpace(req.TargetKey),
		Color:       strings.TrimSpace(req.Color),
		TextColor:   strings.TrimSpace(req.TextColor),
		SortOrder:   req.SortOrder,
		Enabled:     req.Enabled,
		Description: strings.TrimSpace(req.Description),
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, tag)
}

// DeleteTag 删除标签（软删除）。
func (h *TagHandler) DeleteTag(ctx *gin.Context) {
	var uri tagIDURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.tagUseCase == nil {
		InternalError(ctx, "tag service not configured")
		return
	}

	if err := h.tagUseCase.Delete(ctx.Request.Context(), actorFromContext(ctx), uri.TagID); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}
