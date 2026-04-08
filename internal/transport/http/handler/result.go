package handler

import (
	"errors"
	"net/http"

	"omniflow-go/internal/authz"
	"omniflow-go/internal/transport/http/middleware"
	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

const (
	SuccessCode         = "0"
	ClientErrorCode     = "A000001"
	UnauthorizedCode    = "A00200"
	PermissionDenied    = "A000403"
	ServiceErrorCode    = "B000001"
	defaultSuccessMsg   = "success"
	defaultInternalMsg  = "internal server error"
	defaultBadRequest   = "invalid request parameters"
	defaultUnauthorzied = "user token validation failed"
)

type Result struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

func Success(ctx *gin.Context, data any) {
	respond(ctx, http.StatusOK, SuccessCode, defaultSuccessMsg, data)
}

func SuccessNoData(ctx *gin.Context) {
	Success(ctx, nil)
}

func BadRequest(ctx *gin.Context, message string) {
	if message == "" {
		message = defaultBadRequest
	}
	respond(ctx, http.StatusBadRequest, ClientErrorCode, message, nil)
}

func Unauthorized(ctx *gin.Context, message string) {
	if message == "" {
		message = defaultUnauthorzied
	}
	respond(ctx, http.StatusUnauthorized, UnauthorizedCode, message, nil)
}

func Forbidden(ctx *gin.Context, message string) {
	if message == "" {
		message = "permission denied"
	}
	respond(ctx, http.StatusForbidden, PermissionDenied, message, nil)
}

func InternalError(ctx *gin.Context, message string) {
	if message == "" {
		message = defaultInternalMsg
	}
	respond(ctx, http.StatusInternalServerError, ServiceErrorCode, message, nil)
}

func HandleUseCaseError(ctx *gin.Context, err error) {
	if err == nil {
		return
	}

	switch {
	case errors.Is(err, usecase.ErrInvalidArgument):
		BadRequest(ctx, err.Error())
	case errors.Is(err, usecase.ErrUnauthorized):
		Unauthorized(ctx, "")
	case errors.Is(err, authz.ErrPermissionDenied):
		Forbidden(ctx, "")
	case errors.Is(err, usecase.ErrForbidden):
		Forbidden(ctx, "")
	case errors.Is(err, usecase.ErrNotFound):
		respond(ctx, http.StatusNotFound, ClientErrorCode, err.Error(), nil)
	case errors.Is(err, usecase.ErrConflict):
		respond(ctx, http.StatusConflict, ClientErrorCode, err.Error(), nil)
	default:
		InternalError(ctx, err.Error())
	}
}

func respond(ctx *gin.Context, statusCode int, code, message string, data any) {
	ctx.JSON(statusCode, Result{
		Code:      code,
		Message:   message,
		Data:      data,
		RequestID: ctx.GetString(middleware.RequestIDKey),
	})
}
