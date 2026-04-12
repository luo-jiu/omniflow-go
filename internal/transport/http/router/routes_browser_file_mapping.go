package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerBrowserFileMappingRoutes(api *gin.RouterGroup, browserFileMappingHandler *handler.BrowserFileMappingHandler) {
	api.GET("/browser-file-mappings", browserFileMappingHandler.List)
	api.GET("/browser-file-mappings/resolve", browserFileMappingHandler.Resolve)
	api.POST("/browser-file-mappings", browserFileMappingHandler.Create)
	api.PUT("/browser-file-mappings/:mappingId", browserFileMappingHandler.Update)
	api.DELETE("/browser-file-mappings/:mappingId", browserFileMappingHandler.Delete)
}
