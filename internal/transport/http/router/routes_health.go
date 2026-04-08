package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerHealthRoutes(api *gin.RouterGroup, healthHandler *handler.HealthHandler) {
	// 健康检查
	api.GET("/health", healthHandler.Check)
}
