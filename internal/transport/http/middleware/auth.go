package middleware

import (
	"context"
	"net/http"
	"strings"

	"omniflow-go/internal/actor"

	"github.com/gin-gonic/gin"
)

const ActorKey = "actor"

const (
	unauthorizedCode    = "A00200"
	unauthorizedMessage = "user token validation failed"
)

type AuthOptions struct {
	IgnorePaths   []string
	Authenticator TokenAuthenticator
}

type TokenAuthenticator interface {
	AuthenticateActor(ctx context.Context, username, token string) (actor.Actor, error)
}

type result struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

func Auth(options AuthOptions) gin.HandlerFunc {
	ignorePaths := map[string]struct{}{}
	for _, path := range options.IgnorePaths {
		ignorePaths[path] = struct{}{}
	}

	return func(ctx *gin.Context) {
		requestPath := ctx.Request.URL.Path
		if _, ok := ignorePaths[requestPath]; ok {
			ctx.Set(ActorKey, actor.Anonymous())
			ctx.Next()
			return
		}

		token, ok := extractBearerToken(ctx.GetHeader("Authorization"))
		if !ok {
			abortUnauthorized(ctx)
			return
		}

		username := strings.TrimSpace(ctx.GetHeader("username"))
		if username == "" {
			abortUnauthorized(ctx)
			return
		}

		if options.Authenticator != nil {
			principal, err := options.Authenticator.AuthenticateActor(ctx.Request.Context(), username, token)
			if err != nil || principal.IsZero() {
				abortUnauthorized(ctx)
				return
			}
			ctx.Set(ActorKey, principal)
			ctx.Set("access_token", token)
			ctx.Next()
			return
		}

		ctx.Set(ActorKey, actor.Actor{
			ID:     username,
			Name:   username,
			Kind:   actor.KindUser,
			Source: "http-header",
			Scopes: []string{"bearer"},
		})
		ctx.Set("access_token", token)
		ctx.Next()
	}
}

func ActorFromContext(ctx *gin.Context) actor.Actor {
	raw, ok := ctx.Get(ActorKey)
	if !ok {
		return actor.Anonymous()
	}

	principal, ok := raw.(actor.Actor)
	if !ok || principal.IsZero() {
		return actor.Anonymous()
	}
	return principal
}

func extractBearerToken(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) < len("Bearer x") {
		return "", false
	}

	if !strings.EqualFold(value[:7], "Bearer ") {
		return "", false
	}

	token := strings.TrimSpace(value[7:])
	if token == "" {
		return "", false
	}
	return token, true
}

func abortUnauthorized(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusUnauthorized, result{
		Code:      unauthorizedCode,
		Message:   unauthorizedMessage,
		Data:      nil,
		RequestID: ctx.GetString(RequestIDKey),
	})
}
