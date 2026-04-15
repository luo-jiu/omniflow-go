package handler

import (
	"strings"

	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

type BrowserBookmarkHandler struct {
	useCase *usecase.BrowserBookmarkUseCase
}

func NewBrowserBookmarkHandler(useCase *usecase.BrowserBookmarkUseCase) *BrowserBookmarkHandler {
	return &BrowserBookmarkHandler{useCase: useCase}
}

type browserBookmarkIDURI struct {
	BookmarkID uint64 `uri:"bookmarkId" binding:"required"`
}

type browserBookmarkMatchQuery struct {
	URL string `form:"url"`
}

type browserBookmarkCreateRequest struct {
	ParentID *uint64 `json:"parentId"`
	Kind     string  `json:"kind"`
	Title    string  `json:"title" binding:"required"`
	URL      string  `json:"url"`
	IconURL  string  `json:"iconUrl"`
}

type browserBookmarkUpdateRequest struct {
	Title   *string `json:"title"`
	URL     *string `json:"url"`
	IconURL *string `json:"iconUrl"`
}

type browserBookmarkMoveRequest struct {
	ParentID *uint64 `json:"parentId"`
	BeforeID *uint64 `json:"beforeId"`
	AfterID  *uint64 `json:"afterId"`
}

type browserBookmarkImportItemRequest struct {
	Kind     string                             `json:"kind"`
	Title    string                             `json:"title" binding:"required"`
	URL      string                             `json:"url"`
	IconURL  string                             `json:"iconUrl"`
	Children []browserBookmarkImportItemRequest `json:"children"`
}

type browserBookmarkImportRequest struct {
	Source string                             `json:"source"`
	Items  []browserBookmarkImportItemRequest `json:"items" binding:"required"`
}

func (h *BrowserBookmarkHandler) Tree(ctx *gin.Context) {
	if h.useCase == nil {
		InternalError(ctx, "browser bookmark service not configured")
		return
	}

	rows, err := h.useCase.ListTree(ctx.Request.Context(), usecase.ListBrowserBookmarksQuery{
		Actor: actorFromContext(ctx),
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, rows)
}

func (h *BrowserBookmarkHandler) Match(ctx *gin.Context) {
	var query browserBookmarkMatchQuery
	if !BindQuery(ctx, &query) {
		return
	}
	if !RequireNonEmpty(ctx, query.URL, "url") {
		return
	}
	if h.useCase == nil {
		InternalError(ctx, "browser bookmark service not configured")
		return
	}

	result, err := h.useCase.Match(ctx.Request.Context(), usecase.MatchBrowserBookmarkQuery{
		Actor: actorFromContext(ctx),
		URL:   strings.TrimSpace(query.URL),
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, result)
}

func (h *BrowserBookmarkHandler) Create(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var req browserBookmarkCreateRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if !RequireNonEmpty(ctx, req.Title, "title") {
		return
	}
	if h.useCase == nil {
		InternalError(ctx, "browser bookmark service not configured")
		return
	}

	row, err := h.useCase.Create(ctx.Request.Context(), usecase.CreateBrowserBookmarkCommand{
		Actor:    actorFromContext(ctx),
		ParentID: normalizeOptionalUint64Ptr(req.ParentID),
		Kind:     strings.TrimSpace(req.Kind),
		Title:    strings.TrimSpace(req.Title),
		URL:      strings.TrimSpace(req.URL),
		IconURL:  strings.TrimSpace(req.IconURL),
		DryRun:   dryRun,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessWithDryRun(ctx, dryRun, row)
}

func (h *BrowserBookmarkHandler) Update(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var uri browserBookmarkIDURI
	if !BindURI(ctx, &uri) {
		return
	}

	var req browserBookmarkUpdateRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if h.useCase == nil {
		InternalError(ctx, "browser bookmark service not configured")
		return
	}

	row, err := h.useCase.Update(ctx.Request.Context(), uri.BookmarkID, usecase.UpdateBrowserBookmarkCommand{
		Actor:   actorFromContext(ctx),
		Title:   trimOptionalString(req.Title),
		URL:     trimOptionalString(req.URL),
		IconURL: trimOptionalString(req.IconURL),
		DryRun:  dryRun,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessWithDryRun(ctx, dryRun, row)
}

func (h *BrowserBookmarkHandler) Move(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var uri browserBookmarkIDURI
	if !BindURI(ctx, &uri) {
		return
	}

	var req browserBookmarkMoveRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if h.useCase == nil {
		InternalError(ctx, "browser bookmark service not configured")
		return
	}

	row, err := h.useCase.Move(ctx.Request.Context(), usecase.MoveBrowserBookmarkCommand{
		Actor:    actorFromContext(ctx),
		ID:       uri.BookmarkID,
		ParentID: normalizeOptionalUint64Ptr(req.ParentID),
		BeforeID: normalizeOptionalUint64Ptr(req.BeforeID),
		AfterID:  normalizeOptionalUint64Ptr(req.AfterID),
		DryRun:   dryRun,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessWithDryRun(ctx, dryRun, row)
}

func (h *BrowserBookmarkHandler) Delete(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var uri browserBookmarkIDURI
	if !BindURI(ctx, &uri) {
		return
	}
	if h.useCase == nil {
		InternalError(ctx, "browser bookmark service not configured")
		return
	}

	if err := h.useCase.Delete(ctx.Request.Context(), usecase.DeleteBrowserBookmarkCommand{
		Actor:  actorFromContext(ctx),
		ID:     uri.BookmarkID,
		DryRun: dryRun,
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoDataWithDryRun(ctx, dryRun)
}

func (h *BrowserBookmarkHandler) Import(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var req browserBookmarkImportRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if h.useCase == nil {
		InternalError(ctx, "browser bookmark service not configured")
		return
	}

	result, err := h.useCase.Import(ctx.Request.Context(), usecase.ImportBrowserBookmarksCommand{
		Actor:  actorFromContext(ctx),
		Source: strings.TrimSpace(req.Source),
		Items:  toImportBrowserBookmarkItems(req.Items),
		DryRun: dryRun,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessWithDryRun(ctx, dryRun, result)
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func normalizeOptionalUint64Ptr(value *uint64) *uint64 {
	if value == nil || *value == 0 {
		return nil
	}
	return value
}

func toImportBrowserBookmarkItems(items []browserBookmarkImportItemRequest) []usecase.ImportBrowserBookmarkItem {
	result := make([]usecase.ImportBrowserBookmarkItem, 0, len(items))
	for _, item := range items {
		result = append(result, usecase.ImportBrowserBookmarkItem{
			Kind:     strings.TrimSpace(item.Kind),
			Title:    strings.TrimSpace(item.Title),
			URL:      strings.TrimSpace(item.URL),
			IconURL:  strings.TrimSpace(item.IconURL),
			Children: toImportBrowserBookmarkItems(item.Children),
		})
	}
	return result
}
