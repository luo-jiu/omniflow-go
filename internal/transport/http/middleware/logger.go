package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		path := ctx.Request.URL.Path
		query := ctx.Request.URL.RawQuery

		ctx.Next()

		logger.Info("http request",
			"method", ctx.Request.Method,
			"path", path,
			"query", query,
			"status", ctx.Writer.Status(),
			"latency", time.Since(start),
			"client_ip", ctx.ClientIP(),
			"request_id", ctx.GetString(RequestIDKey),
		)
	}
}
