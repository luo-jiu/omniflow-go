package handler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func BindJSON(ctx *gin.Context, req any) bool {
	if err := ctx.ShouldBindJSON(req); err != nil {
		BadRequest(ctx, fmt.Sprintf("invalid json body: %v", err))
		return false
	}
	return true
}

func BindQuery(ctx *gin.Context, req any) bool {
	if err := ctx.ShouldBindQuery(req); err != nil {
		BadRequest(ctx, fmt.Sprintf("invalid query params: %v", err))
		return false
	}
	return true
}

func BindURI(ctx *gin.Context, req any) bool {
	if err := ctx.ShouldBindUri(req); err != nil {
		BadRequest(ctx, fmt.Sprintf("invalid uri params: %v", err))
		return false
	}
	return true
}

func BindForm(ctx *gin.Context, req any) bool {
	if err := ctx.ShouldBind(req); err != nil {
		BadRequest(ctx, fmt.Sprintf("invalid form params: %v", err))
		return false
	}
	return true
}

func RequireNonEmpty(ctx *gin.Context, value, fieldName string) bool {
	if strings.TrimSpace(value) == "" {
		BadRequest(ctx, fmt.Sprintf("%s is required", fieldName))
		return false
	}
	return true
}

func QueryString(ctx *gin.Context, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(ctx.Query(key)); v != "" {
			return v
		}
	}
	return ""
}

func PostFormString(ctx *gin.Context, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(ctx.PostForm(key)); v != "" {
			return v
		}
	}
	return ""
}

func QueryUint64(ctx *gin.Context, required bool, keys ...string) (uint64, bool) {
	raw := QueryString(ctx, keys...)
	return parseUint64(ctx, raw, keys, required)
}

func PostFormUint64(ctx *gin.Context, required bool, keys ...string) (uint64, bool) {
	raw := PostFormString(ctx, keys...)
	return parseUint64(ctx, raw, keys, required)
}

func QueryInt(ctx *gin.Context, defaultValue int, required bool, keys ...string) (int, bool) {
	raw := QueryString(ctx, keys...)
	fieldName := firstKeyOrDefault(keys, "query")
	if raw == "" {
		if required {
			BadRequest(ctx, fmt.Sprintf("%s is required", fieldName))
			return 0, false
		}
		return defaultValue, true
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		BadRequest(ctx, fmt.Sprintf("%s must be integer", fieldName))
		return 0, false
	}
	return value, true
}

func QueryBool(ctx *gin.Context, defaultValue bool, keys ...string) (bool, bool) {
	raw := QueryString(ctx, keys...)
	fieldName := firstKeyOrDefault(keys, "query")
	if raw == "" {
		return defaultValue, true
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		BadRequest(ctx, fmt.Sprintf("%s must be boolean", fieldName))
		return false, false
	}
	return value, true
}

func parseUint64(ctx *gin.Context, raw string, keys []string, required bool) (uint64, bool) {
	fieldName := firstKeyOrDefault(keys, "value")
	if raw == "" {
		if required {
			BadRequest(ctx, fmt.Sprintf("%s is required", fieldName))
			return 0, false
		}
		return 0, true
	}

	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		BadRequest(ctx, fmt.Sprintf("%s must be positive integer", fieldName))
		return 0, false
	}
	return value, true
}

func firstKeyOrDefault(keys []string, fallback string) string {
	if len(keys) == 0 || strings.TrimSpace(keys[0]) == "" {
		return fallback
	}
	return keys[0]
}
