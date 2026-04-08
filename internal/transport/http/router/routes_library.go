package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerLibraryRoutes(api *gin.RouterGroup, libraryHandler *handler.LibraryHandler) {
	// 资料库
	api.GET("/libraries/scroll", libraryHandler.Scroll)
	api.POST("/libraries", libraryHandler.Create)
	api.PUT("/libraries/:id", libraryHandler.Update)
	api.DELETE("/libraries/:id", libraryHandler.Delete)
}
