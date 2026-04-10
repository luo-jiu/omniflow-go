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

// Register 保留健康检查路由的兼容注册方式。
func (h *HealthHandler) Register(routes gin.IRouter) {
	routes.GET("/health", h.Check)
}

// Check 返回服务健康状态。
func (h *HealthHandler) Check(ctx *gin.Context) {
	if h.healthUseCase == nil {
		InternalError(ctx, "health service not configured")
		return
	}
	Success(ctx, h.healthUseCase.Status(ctx.Request.Context()))
}
