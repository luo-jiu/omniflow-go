package handler

import (
	"strings"

	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userUseCase *usecase.UserUseCase
}

func NewUserHandler(userUseCase *usecase.UserUseCase) *UserHandler {
	return &UserHandler{userUseCase: userUseCase}
}

func (h *UserHandler) Register(group *gin.RouterGroup) {
	group.GET("/exists", h.HasUsername)
	group.GET("/:username", h.GetActualUserByUsername)
	group.POST("", h.RegisterUser)
	group.PUT("/:id", h.UpdateUser)
}

type userNameURI struct {
	Username string `uri:"username" binding:"required"`
}

type userIDURI struct {
	ID uint64 `uri:"id" binding:"required"`
}

type usernameExistsQuery struct {
	Username string `form:"username" binding:"required"`
}

type registerUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
}

type updateUserRequest struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
	Phone    *string `json:"phone"`
	Email    *string `json:"email"`
}

func (h *UserHandler) GetActualUserByUsername(ctx *gin.Context) {
	var uri userNameURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.userUseCase == nil {
		Success(ctx, map[string]any{"username": uri.Username})
		return
	}

	user, err := h.userUseCase.GetByUsername(ctx.Request.Context(), uri.Username)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, user)
}

func (h *UserHandler) HasUsername(ctx *gin.Context) {
	var query usernameExistsQuery
	if !BindQuery(ctx, &query) {
		return
	}

	if h.userUseCase == nil {
		Success(ctx, false)
		return
	}

	ok, err := h.userUseCase.Exists(ctx.Request.Context(), query.Username)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, ok)
}

func (h *UserHandler) RegisterUser(ctx *gin.Context) {
	var req registerUserRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if !RequireNonEmpty(ctx, strings.TrimSpace(req.Username), "username") {
		return
	}
	if !RequireNonEmpty(ctx, strings.TrimSpace(req.Password), "password") {
		return
	}

	if h.userUseCase == nil {
		SuccessNoData(ctx)
		return
	}

	created, err := h.userUseCase.Register(ctx.Request.Context(), usecase.RegisterUserCommand{
		Actor:    actorFromContext(ctx),
		Username: req.Username,
		Password: req.Password,
		Phone:    req.Phone,
		Email:    req.Email,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, created)
}

func (h *UserHandler) UpdateUser(ctx *gin.Context) {
	var uri userIDURI
	if !BindURI(ctx, &uri) {
		return
	}

	var req updateUserRequest
	if !BindJSON(ctx, &req) {
		return
	}

	if h.userUseCase == nil {
		SuccessNoData(ctx)
		return
	}

	updated, err := h.userUseCase.Update(ctx.Request.Context(), usecase.UpdateUserCommand{
		Actor:    actorFromContext(ctx),
		ID:       uri.ID,
		Username: req.Username,
		Password: req.Password,
		Phone:    req.Phone,
		Email:    req.Email,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, updated)
}
