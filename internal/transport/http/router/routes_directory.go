package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerDirectoryRoutes(api *gin.RouterGroup, directoryHandler *handler.DirectoryHandler) {
	// 目录上传（放宽超时，大文件通过 WiFi 上传到 MinIO 可能超过默认 write_timeout）
	upload := api.Group("/directory")
	upload.Use(extendUploadTimeout())
	upload.POST("/upload", directoryHandler.UploadFile)
	api.PUT("/nodes/:nodeId/content", directoryHandler.UpdateFileContent)
	api.GET("/directory/link", directoryHandler.GetFileLink)
	api.POST("/directory/links/batch", directoryHandler.BatchGetFileLinks)
}
