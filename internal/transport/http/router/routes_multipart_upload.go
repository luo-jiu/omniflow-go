package router

import (
	"net/http"
	"time"

	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerMultipartUploadRoutes(api *gin.RouterGroup, h *handler.MultipartUploadHandler) {
	mp := api.Group("/directory/upload/multipart")
	mp.Use(extendUploadTimeout())
	mp.POST("/initiate", h.InitiateUpload)
	mp.PUT("/:uploadId/part/:partNumber", h.UploadPart)
	mp.POST("/:uploadId/complete", h.CompleteUpload)
	mp.DELETE("/:uploadId", h.AbortUpload)
	mp.GET("/:uploadId/parts", h.ListParts)
}

func extendUploadTimeout() gin.HandlerFunc {
	return func(c *gin.Context) {
		rc := http.NewResponseController(c.Writer)
		_ = rc.SetReadDeadline(time.Now().Add(5 * time.Minute))
		_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Minute))
		c.Next()
	}
}
