package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerResourceMonitorRoutes(api *gin.RouterGroup, h *handler.ResourceMonitorHandler) {
	group := api.Group("/resource-monitor")
	group.GET("/snapshot", h.Snapshot)
	group.POST("/samples", h.CaptureSample)
}
