package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerUserRoutes(api *gin.RouterGroup, userHandler *handler.UserHandler) {
	// 用户
	api.GET("/user/exists", userHandler.HasUsername)
	api.GET("/user/:username", userHandler.GetActualUserByUsername)
	api.POST("/user", userHandler.RegisterUser)
	api.PUT("/user/:id", userHandler.UpdateUser)
}
