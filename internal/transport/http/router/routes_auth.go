package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerAuthRoutes(api *gin.RouterGroup, authHandler *handler.AuthHandler) {
	// 认证
	api.POST("/auth/login", authHandler.Login)
	api.GET("/auth/status", authHandler.Status)
	api.DELETE("/auth/logout", authHandler.Logout)
}
