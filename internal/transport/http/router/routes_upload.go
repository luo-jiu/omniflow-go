package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

// registerUploadRoutes 挂载直传 MinIO 流程的 7 个端点。
// 与 proxy 时代不同，所有字节流量直接打到 MinIO，后端只编排 init/sign/complete/abort/renew。
func registerUploadRoutes(api *gin.RouterGroup, h *handler.UploadHandler) {
	g := api.Group("/upload")
	g.POST("/init", h.Init)
	g.POST("/parts/sign", h.SignParts)
	g.GET("/parts", h.ListParts)
	g.POST("/:uploadId/renew", h.Renew)
	g.POST("/complete", h.Complete)
	g.DELETE("/:uploadId", h.Abort)
}
