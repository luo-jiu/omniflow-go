package middleware

import (
	"log/slog"
	"strings"
	"time"

	"omniflow-go/internal/actor"

	"github.com/gin-gonic/gin"
)

func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		path := ctx.Request.URL.Path
		query := ctx.Request.URL.RawQuery

		ctx.Next()

		principal := ActorFromContext(ctx)
		actorID := principal.ID
		actorKind := principal.Kind
		if principal.IsZero() {
			actorID = actor.Anonymous().ID
			actorKind = actor.Anonymous().Kind
		}

		args := []any{
			"method", ctx.Request.Method,
			"path", path,
			"query", query,
			"status", ctx.Writer.Status(),
			"latency", time.Since(start),
			"client_ip", ctx.ClientIP(),
			"request_id", ctx.GetString(RequestIDKey),
			"actor_id", actorID,
			"actor_kind", actorKind,
			"dry_run", strings.EqualFold(ctx.Writer.Header().Get("X-Omniflow-Dry-Run"), "true"),
		}
		if len(ctx.Errors) > 0 {
			args = append(args, "error_count", len(ctx.Errors))
			args = append(args, "errors", strings.TrimSpace(ctx.Errors.String()))
		}

		logger.Info("http request", args...)
	}
}
