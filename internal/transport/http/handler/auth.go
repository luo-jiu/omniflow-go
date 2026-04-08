package handler

import (
	"context"
	"strings"

	"omniflow-go/internal/actor"
	domainuser "omniflow-go/internal/domain/user"
	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authUseCase *usecase.AuthUseCase
}

func NewAuthHandler(authUseCase *usecase.AuthUseCase) *AuthHandler {
	return &AuthHandler{authUseCase: authUseCase}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type authStatusQuery struct {
	Username string `form:"username" binding:"required"`
	Token    string `form:"token" binding:"required"`
}

type loginResponse struct {
	Token    string         `json:"token"`
	Username string         `json:"username"`
	UserInfo map[string]any `json:"userInfo"`
}

// Login 使用用户名密码登录，并返回 token 与用户信息。
func (h *AuthHandler) Login(ctx *gin.Context) {
	var req loginRequest
	if !BindJSON(ctx, &req) {
		return
	}

	username := strings.TrimSpace(req.Username)
	if !RequireNonEmpty(ctx, username, "username") {
		return
	}

	if h.authUseCase == nil {
		InternalError(ctx, "auth service not configured")
		return
	}

	result, err := h.authUseCase.Login(ctx.Request.Context(), usecase.LoginCommand{
		Actor:    actorFromContext(ctx),
		Username: username,
		Password: req.Password,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}

	Success(ctx, loginResponse{
		Token:    result.Token,
		Username: username,
		UserInfo: map[string]any{
			"username": username,
			"status":   domainuser.StatusActive,
		},
	})
}

// Status 校验 username/token 是否仍处于登录态。
func (h *AuthHandler) Status(ctx *gin.Context) {
	var query authStatusQuery
	if !BindQuery(ctx, &query) {
		return
	}

	if h.authUseCase == nil {
		Success(ctx, false)
		return
	}

	ok, err := h.authUseCase.Check(ctx.Request.Context(), query.Username, query.Token)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, ok)
}

// Logout 注销当前登录 token。
func (h *AuthHandler) Logout(ctx *gin.Context) {
	var query authStatusQuery
	if !BindQuery(ctx, &query) {
		return
	}

	if h.authUseCase == nil {
		SuccessNoData(ctx)
		return
	}

	if err := h.authUseCase.Logout(ctx.Request.Context(), query.Username, query.Token); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// AuthenticateActor 根据请求头和 bearer token 解析当前操作者。
func (h *AuthHandler) AuthenticateActor(ctx context.Context, username, token string) (actor.Actor, error) {
	if h == nil || h.authUseCase == nil || !h.authUseCase.CanAuthenticate() {
		name := strings.TrimSpace(username)
		return actor.Actor{
			ID:     name,
			Name:   name,
			Kind:   actor.KindUser,
			Source: "http-header",
			Scopes: []string{"bearer"},
		}, nil
	}
	return h.authUseCase.ResolveActor(ctx, username, token)
}
