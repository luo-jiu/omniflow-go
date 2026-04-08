package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerUserRoutes(api *gin.RouterGroup, userHandler *handler.UserHandler) {
	// 用户
	api.GET("/user/me", userHandler.GetCurrentUser)
	api.PUT("/user/me", userHandler.UpdateCurrentUser)
	api.PUT("/user/me/password", userHandler.UpdateCurrentUserPassword)
	api.POST("/user/me/avatar", userHandler.UploadCurrentUserAvatar)
	api.GET("/user/exists", userHandler.HasUsername)
	api.GET("/user/:username", userHandler.GetActualUserByUsername)
	api.POST("/user", userHandler.RegisterUser)
	api.PUT("/user/:id", userHandler.UpdateUser)
}
