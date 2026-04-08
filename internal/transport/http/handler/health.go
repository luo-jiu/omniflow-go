package handler

import (
	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	healthUseCase *usecase.HealthUseCase
}

func NewHealthHandler(healthUseCase *usecase.HealthUseCase) *HealthHandler {
	return &HealthHandler{healthUseCase: healthUseCase}
}

func (h *HealthHandler) Register(routes gin.IRouter) {
	routes.GET("/health", h.Check)
}

func (h *HealthHandler) Check(ctx *gin.Context) {
	Success(ctx, h.healthUseCase.Status(ctx.Request.Context()))
}
