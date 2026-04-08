package handler

import (
	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

type TagHandler struct {
	tagUseCase *usecase.TagUseCase
}

func NewTagHandler(tagUseCase *usecase.TagUseCase) *TagHandler {
	return &TagHandler{tagUseCase: tagUseCase}
}

func (h *TagHandler) Register(group *gin.RouterGroup) {
	group.GET("/search-types", h.GetSearchTypes)
}

func (h *TagHandler) GetSearchTypes(ctx *gin.Context) {
	if h.tagUseCase == nil {
		Success(ctx, "PostgreSQL")
		return
	}
	Success(ctx, h.tagUseCase.SearchType())
}
