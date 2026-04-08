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
	Nickname string `json:"nickname"`
	Password string `json:"password" binding:"required"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Ext      string `json:"ext"`
}

type updateUserRequest struct {
	Nickname *string `json:"nickname"`
	Phone    *string `json:"phone"`
	Email    *string `json:"email"`
	Ext      *string `json:"ext"`
}

type updateCurrentPasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

// GetActualUserByUsername 根据用户名获取用户信息。
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

// HasUsername 校验用户名是否已存在。
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

// RegisterUser 注册新用户。
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

	_, err := h.userUseCase.Register(ctx.Request.Context(), usecase.RegisterUserCommand{
		Actor:    actorFromContext(ctx),
		Username: req.Username,
		Nickname: req.Nickname,
		Password: req.Password,
		Phone:    req.Phone,
		Email:    req.Email,
		Ext:      req.Ext,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// UpdateUser 按用户 ID 更新用户资料。
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

	_, err := h.userUseCase.Update(ctx.Request.Context(), usecase.UpdateUserCommand{
		Actor:    actorFromContext(ctx),
		ID:       uri.ID,
		Nickname: req.Nickname,
		Phone:    req.Phone,
		Email:    req.Email,
		Ext:      req.Ext,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// GetCurrentUser 获取当前登录用户资料。
func (h *UserHandler) GetCurrentUser(ctx *gin.Context) {
	if h.userUseCase == nil {
		Success(ctx, map[string]any{})
		return
	}

	user, err := h.userUseCase.GetCurrent(ctx.Request.Context(), actorFromContext(ctx))
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, user)
}

// UpdateCurrentUser 更新当前登录用户资料。
func (h *UserHandler) UpdateCurrentUser(ctx *gin.Context) {
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
		Nickname: req.Nickname,
		Phone:    req.Phone,
		Email:    req.Email,
		Ext:      req.Ext,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, updated)
}

// UpdateCurrentUserPassword 修改当前登录用户密码。
func (h *UserHandler) UpdateCurrentUserPassword(ctx *gin.Context) {
	var req updateCurrentPasswordRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if !RequireNonEmpty(ctx, strings.TrimSpace(req.OldPassword), "oldPassword") {
		return
	}
	if !RequireNonEmpty(ctx, strings.TrimSpace(req.NewPassword), "newPassword") {
		return
	}

	if h.userUseCase == nil {
		SuccessNoData(ctx)
		return
	}

	if err := h.userUseCase.UpdateCurrentPassword(ctx.Request.Context(), usecase.UpdateCurrentPasswordCommand{
		Actor:       actorFromContext(ctx),
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// UploadCurrentUserAvatar 上传并更新当前登录用户头像。
func (h *UserHandler) UploadCurrentUserAvatar(ctx *gin.Context) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		BadRequest(ctx, "file is required")
		return
	}

	if h.userUseCase == nil {
		SuccessNoData(ctx)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		InternalError(ctx, "open upload file failed")
		return
	}
	defer file.Close()

	updated, err := h.userUseCase.UploadCurrentAvatar(ctx.Request.Context(), usecase.UploadCurrentUserAvatarCommand{
		Actor:       actorFromContext(ctx),
		FileName:    fileHeader.Filename,
		FileSize:    fileHeader.Size,
		ContentType: fileHeader.Header.Get("Content-Type"),
		Content:     file,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, updated)
}
