package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerTagRoutes(api *gin.RouterGroup, tagHandler *handler.TagHandler) {
	// 标签
	api.GET("/tags/search-types", tagHandler.GetSearchTypes)
}
