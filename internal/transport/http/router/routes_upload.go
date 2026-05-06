package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

// registerUploadProgressRoutes 注册整传与分片共享的进度查询端点。
// 该端点不挂在 multipart 子分组下，因为整传 uploadID 也走它；
// 也无需 extendUploadTimeout，进度查询是短请求。
func registerUploadProgressRoutes(api *gin.RouterGroup, h *handler.UploadProgressHandler) {
	api.GET("/upload/:uploadId/progress", h.GetProgress)
}
