package middleware

import (
	"context"
	"net/http"
	"strings"

	"omniflow-go/internal/actor"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
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
	ignorePaths := buildIgnorePathSet(options.IgnorePaths)

	return func(ctx *gin.Context) {
		requestPath := ctx.Request.URL.Path
		if ignorePaths.Match(ctx.Request.Method, requestPath) {
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

type ignorePathSet struct {
	pathOnly   map[string]struct{}
	methodPath map[string]struct{}
}

func (s ignorePathSet) Match(method, path string) bool {
	if _, ok := s.methodPath[ignoreMethodPathKey(method, path)]; ok {
		return true
	}
	_, ok := s.pathOnly[path]
	return ok
}

// buildIgnorePathSet 归一化白名单路径并构建集合，支持两种规则：
// 1) "/api/v1/auth/login"（仅按路径）
// 2) "POST /api/v1/auth/login"（按 method+path）
func buildIgnorePathSet(paths []string) ignorePathSet {
	normalized := lo.FilterMap(paths, func(path string, _ int) (string, bool) {
		clean := strings.TrimSpace(path)
		if clean == "" {
			return "", false
		}
		return clean, true
	})

	set := ignorePathSet{
		pathOnly:   map[string]struct{}{},
		methodPath: map[string]struct{}{},
	}
	for _, rule := range lo.Uniq(normalized) {
		method, path, withMethod := parseIgnoreRule(rule)
		if withMethod {
			set.methodPath[ignoreMethodPathKey(method, path)] = struct{}{}
			continue
		}
		set.pathOnly[path] = struct{}{}
	}
	return set
}

func parseIgnoreRule(rule string) (method, path string, withMethod bool) {
	parts := strings.Fields(rule)
	if len(parts) != 2 {
		return "", rule, false
	}

	method = strings.ToUpper(strings.TrimSpace(parts[0]))
	path = strings.TrimSpace(parts[1])
	if method == "" || path == "" || !strings.HasPrefix(path, "/") {
		return "", rule, false
	}
	return method, path, true
}

func ignoreMethodPathKey(method, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + strings.TrimSpace(path)
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
