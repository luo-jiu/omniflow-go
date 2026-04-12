package handler

import (
	"strings"

	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

type BrowserFileMappingHandler struct {
	useCase *usecase.BrowserFileMappingUseCase
}

func NewBrowserFileMappingHandler(useCase *usecase.BrowserFileMappingUseCase) *BrowserFileMappingHandler {
	return &BrowserFileMappingHandler{useCase: useCase}
}

type browserFileMappingIDURI struct {
	MappingID uint64 `uri:"mappingId" binding:"required"`
}

type browserFileMappingResolveQuery struct {
	FileExt string `form:"fileExt"`
}

type browserFileMappingUpsertRequest struct {
	FileExt string `json:"fileExt" binding:"required"`
	SiteURL string `json:"siteUrl" binding:"required"`
}

func (h *BrowserFileMappingHandler) List(ctx *gin.Context) {
	if h.useCase == nil {
		InternalError(ctx, "browser file mapping service not configured")
		return
	}

	rows, err := h.useCase.List(ctx.Request.Context(), usecase.ListBrowserFileMappingsQuery{
		Actor: actorFromContext(ctx),
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, rows)
}

func (h *BrowserFileMappingHandler) Resolve(ctx *gin.Context) {
	var query browserFileMappingResolveQuery
	if !BindQuery(ctx, &query) {
		return
	}
	if !RequireNonEmpty(ctx, query.FileExt, "fileExt") {
		return
	}
	if h.useCase == nil {
		InternalError(ctx, "browser file mapping service not configured")
		return
	}

	row, err := h.useCase.Resolve(ctx.Request.Context(), usecase.ResolveBrowserFileMappingQuery{
		Actor:   actorFromContext(ctx),
		FileExt: strings.TrimSpace(query.FileExt),
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, row)
}

func (h *BrowserFileMappingHandler) Create(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var req browserFileMappingUpsertRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if !RequireNonEmpty(ctx, req.FileExt, "fileExt") || !RequireNonEmpty(ctx, req.SiteURL, "siteUrl") {
		return
	}
	if h.useCase == nil {
		InternalError(ctx, "browser file mapping service not configured")
		return
	}

	row, err := h.useCase.Create(ctx.Request.Context(), usecase.CreateBrowserFileMappingCommand{
		Actor:   actorFromContext(ctx),
		FileExt: strings.TrimSpace(req.FileExt),
		SiteURL: strings.TrimSpace(req.SiteURL),
		DryRun:  dryRun,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessWithDryRun(ctx, dryRun, row)
}

func (h *BrowserFileMappingHandler) Update(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var uri browserFileMappingIDURI
	if !BindURI(ctx, &uri) {
		return
	}

	var req browserFileMappingUpsertRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if !RequireNonEmpty(ctx, req.FileExt, "fileExt") || !RequireNonEmpty(ctx, req.SiteURL, "siteUrl") {
		return
	}
	if h.useCase == nil {
		InternalError(ctx, "browser file mapping service not configured")
		return
	}

	row, err := h.useCase.Update(ctx.Request.Context(), uri.MappingID, usecase.UpdateBrowserFileMappingCommand{
		Actor:   actorFromContext(ctx),
		FileExt: strings.TrimSpace(req.FileExt),
		SiteURL: strings.TrimSpace(req.SiteURL),
		DryRun:  dryRun,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessWithDryRun(ctx, dryRun, row)
}

func (h *BrowserFileMappingHandler) Delete(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var uri browserFileMappingIDURI
	if !BindURI(ctx, &uri) {
		return
	}
	if h.useCase == nil {
		InternalError(ctx, "browser file mapping service not configured")
		return
	}

	if err := h.useCase.Delete(ctx.Request.Context(), usecase.DeleteBrowserFileMappingCommand{
		Actor:     actorFromContext(ctx),
		MappingID: uri.MappingID,
		DryRun:    dryRun,
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoDataWithDryRun(ctx, dryRun)
}
