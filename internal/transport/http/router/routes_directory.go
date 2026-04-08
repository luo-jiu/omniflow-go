package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerDirectoryRoutes(api *gin.RouterGroup, directoryHandler *handler.DirectoryHandler) {
	// 目录上传和直链
	api.POST("/directory/upload", directoryHandler.UploadFile)
	api.GET("/directory/link", directoryHandler.GetFileLink)
}
