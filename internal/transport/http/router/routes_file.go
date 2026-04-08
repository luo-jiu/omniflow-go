package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerFileRoutes(api *gin.RouterGroup, fileHandler *handler.FileHandler) {
	// 文件上传和直链
	api.POST("/files/upload", fileHandler.UploadFile)
	api.GET("/files/link", fileHandler.GetFileLink)
}
