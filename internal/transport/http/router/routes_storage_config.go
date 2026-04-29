package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerStorageConfigRoutes(api *gin.RouterGroup, h *handler.StorageConfigHandler) {
	sg := api.Group("/storage")
	sg.GET("/providers", h.ListProviders)
	sg.POST("/providers", h.AddProvider)
	sg.PUT("/providers/:alias", h.UpdateProvider)
	sg.DELETE("/providers/:alias", h.DeleteProvider)
	sg.POST("/providers/:alias/test", h.TestProvider)
	sg.GET("/default", h.GetDefault)
	sg.PUT("/default", h.SetDefault)
	sg.GET("/routing-rules", h.GetRoutingRules)
	sg.PUT("/routing-rules", h.UpdateRoutingRules)
	sg.POST("/resolve-target", h.ResolveTarget)
}
