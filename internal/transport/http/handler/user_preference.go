package handler

import (
	"encoding/json"

	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

type userPreferenceNamespaceURI struct {
	Namespace string `uri:"namespace" binding:"required"`
}

type upsertUserPreferenceRequest struct {
	Preferences      json.RawMessage `json:"preferences" binding:"required"`
	SchemaVersion    int32           `json:"schemaVersion" binding:"required"`
	ExpectedRevision int64           `json:"expectedRevision"`
}

// ListCurrentUserPreferences 获取当前用户的全部跨设备偏好。
func (h *UserHandler) ListCurrentUserPreferences(ctx *gin.Context) {
	if h.preferenceUseCase == nil {
		InternalError(ctx, "user preference service not configured")
		return
	}

	preferences, err := h.preferenceUseCase.List(ctx.Request.Context(), usecase.ListUserPreferencesQuery{
		Actor: actorFromContext(ctx),
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, preferences)
}

// GetCurrentUserPreference 获取当前用户指定命名空间的跨设备偏好。
func (h *UserHandler) GetCurrentUserPreference(ctx *gin.Context) {
	var uri userPreferenceNamespaceURI
	if !BindURI(ctx, &uri) {
		return
	}
	if h.preferenceUseCase == nil {
		InternalError(ctx, "user preference service not configured")
		return
	}

	preference, err := h.preferenceUseCase.Get(ctx.Request.Context(), usecase.GetUserPreferenceQuery{
		Actor:     actorFromContext(ctx),
		Namespace: uri.Namespace,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, preference)
}

// UpsertCurrentUserPreference 保存当前用户指定命名空间的跨设备偏好。
func (h *UserHandler) UpsertCurrentUserPreference(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var uri userPreferenceNamespaceURI
	if !BindURI(ctx, &uri) {
		return
	}
	var req upsertUserPreferenceRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if h.preferenceUseCase == nil {
		InternalError(ctx, "user preference service not configured")
		return
	}

	preference, err := h.preferenceUseCase.Upsert(ctx.Request.Context(), usecase.UpsertUserPreferenceCommand{
		Actor:            actorFromContext(ctx),
		Namespace:        uri.Namespace,
		Preferences:      req.Preferences,
		SchemaVersion:    req.SchemaVersion,
		ExpectedRevision: req.ExpectedRevision,
		DryRun:           dryRun,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessWithDryRun(ctx, dryRun, preference)
}
