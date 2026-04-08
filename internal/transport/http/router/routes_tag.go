package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerTagRoutes(api *gin.RouterGroup, tagHandler *handler.TagHandler) {
	// 标签
	api.GET("/tags/search-types", tagHandler.GetSearchTypes)
	api.GET("/tags", tagHandler.ListTags)
	api.POST("/tags", tagHandler.CreateTag)
	api.PUT("/tags/:tagId", tagHandler.UpdateTag)
	api.DELETE("/tags/:tagId", tagHandler.DeleteTag)
}
