package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerDirectoryRoutes(api *gin.RouterGroup, directoryHandler *handler.DirectoryHandler) {
	api.PUT("/nodes/:nodeId/content", directoryHandler.UpdateFileContent)
	api.GET("/directory/link", directoryHandler.GetFileLink)
	api.POST("/directory/links/batch", directoryHandler.BatchGetFileLinks)
}
