package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	domainauth "omniflow-go/internal/domain/auth"
	domainuser "omniflow-go/internal/domain/user"
	"omniflow-go/internal/repository"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type LoginCommand struct {
	Actor    actor.Actor
	Username string
	Password string
}

type LoginResult struct {
	Token string
	User  domainuser.User
}

type AuthUseCase struct {
	users    *repository.UserRepository
	sessions domainauth.SessionStore
	auditLog audit.Sink
}

// NewAuthUseCase 构建认证用例，依赖用户仓储与会话仓储。
func NewAuthUseCase(users *repository.UserRepository, sessions domainauth.SessionStore, auditLog audit.Sink) *AuthUseCase {
	return &AuthUseCase{
		users:    users,
		sessions: sessions,
		auditLog: auditLog,
	}
}

func (u *AuthUseCase) Login(ctx context.Context, cmd LoginCommand) (LoginResult, error) {
	username := strings.TrimSpace(cmd.Username)
	password := strings.TrimSpace(cmd.Password)
	if username == "" || password == "" {
		slog.WarnContext(ctx, "auth.login.invalid_argument",
			"username", username,
			"has_password", password != "",
		)
		return LoginResult{}, fmt.Errorf("%w: username and password are required", ErrInvalidArgument)
	}

	if u.sessions == nil {
		return LoginResult{}, fmt.Errorf("%w: session store not configured", ErrInvalidArgument)
	}
	if u.users == nil {
		return LoginResult{}, fmt.Errorf("%w: user repository not configured", ErrInvalidArgument)
	}

	authUser, err := u.users.FindActiveByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			slog.InfoContext(ctx, "auth.login.failed",
				"username", username,
				"reason", "user_not_found",
			)
			_ = u.RecordAttempt(ctx, cmd.Actor, false)
			return LoginResult{}, fmt.Errorf("%w: username or password is invalid", ErrInvalidCredentials)
		}
		return LoginResult{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(authUser.PasswordHash), []byte(password)); err != nil {
		slog.InfoContext(ctx, "auth.login.failed",
			"username", username,
			"reason", "password_mismatch",
		)
		_ = u.RecordAttempt(ctx, cmd.Actor, false)
		return LoginResult{}, fmt.Errorf("%w: username or password is invalid", ErrInvalidCredentials)
	}

	token, err := u.sessions.FindFirstToken(ctx, username)
	if err != nil {
		return LoginResult{}, err
	}
	if token != "" {
		userInfo := normalizeLoginUser(authUser.User)
		slog.InfoContext(ctx, "auth.login.succeeded",
			"username", username,
			"user_id", userInfo.ID,
			"cached_session", true,
		)
		_ = u.RecordAttempt(ctx, cmd.Actor, true)
		_ = u.writeAudit(ctx, cmd.Actor, "auth.login", true, map[string]any{
			"username": username,
			"token":    token,
			"cached":   true,
		})
		return LoginResult{
			Token: token,
			User:  userInfo,
		}, nil
	}

	token = uuid.NewString()
	if err := u.sessions.Save(ctx, username, token, authUser.User); err != nil {
		return LoginResult{}, err
	}

	userInfo := normalizeLoginUser(authUser.User)
	slog.InfoContext(ctx, "auth.login.succeeded",
		"username", username,
		"user_id", userInfo.ID,
		"cached_session", false,
	)
	_ = u.RecordAttempt(ctx, cmd.Actor, true)
	_ = u.writeAudit(ctx, cmd.Actor, "auth.login", true, map[string]any{
		"username": username,
		"token":    token,
		"cached":   false,
	})
	return LoginResult{
		Token: token,
		User:  userInfo,
	}, nil
}

func (u *AuthUseCase) Check(ctx context.Context, username, token string) (bool, error) {
	username = strings.TrimSpace(username)
	token = strings.TrimSpace(token)
	if username == "" || token == "" {
		// 对齐 Java 黑盒语义：参数为空时直接返回未登录，而不是参数错误。
		slog.DebugContext(ctx, "auth.status.checked",
			"username", username,
			"result", false,
			"reason", "missing_argument",
		)
		return false, nil
	}

	if u.sessions == nil {
		return false, fmt.Errorf("%w: session store not configured", ErrInvalidArgument)
	}

	ok, err := u.sessions.Exists(ctx, username, token)
	if err != nil {
		return false, err
	}
	slog.DebugContext(ctx, "auth.status.checked",
		"username", username,
		"result", ok,
		"reason", "session_lookup",
	)
	return ok, nil
}

func (u *AuthUseCase) ResolveActor(ctx context.Context, username, token string) (actor.Actor, error) {
	username = strings.TrimSpace(username)
	token = strings.TrimSpace(token)
	if username == "" || token == "" {
		return actor.Actor{}, fmt.Errorf("%w: username and token are required", ErrInvalidArgument)
	}

	if u.sessions == nil {
		return actor.Actor{}, fmt.Errorf("%w: session store not configured", ErrInvalidArgument)
	}
	if u.users == nil {
		return actor.Actor{}, fmt.Errorf("%w: user repository not configured", ErrInvalidArgument)
	}

	sessionUser, err := u.sessions.GetUser(ctx, username, token)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return actor.Actor{}, ErrUnauthorized
		}
		return actor.Actor{}, err
	}

	if sessionUser.ID == 0 {
		found, err := u.users.FindActiveByUsername(ctx, username)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return actor.Actor{}, ErrUnauthorized
			}
			return actor.Actor{}, err
		}
		sessionUser = found.User
	}

	name := strings.TrimSpace(sessionUser.Username)
	if name == "" {
		name = username
	}

	return actor.Actor{
		ID:     strconv.FormatUint(sessionUser.ID, 10),
		Name:   name,
		Kind:   actor.KindUser,
		Source: "session",
		Scopes: []string{"bearer"},
	}, nil
}

func (u *AuthUseCase) Logout(ctx context.Context, username, token string, dryRun bool) error {
	username = strings.TrimSpace(username)
	token = strings.TrimSpace(token)
	if username == "" || token == "" {
		slog.WarnContext(ctx, "auth.logout.invalid_argument",
			"username", username,
			"has_token", token != "",
			"dry_run", dryRun,
		)
		return fmt.Errorf("%w: username and token are required", ErrInvalidArgument)
	}

	if u.sessions == nil {
		return fmt.Errorf("%w: session store not configured", ErrInvalidArgument)
	}

	if dryRun {
		exists, err := u.sessions.Exists(ctx, username, token)
		if err != nil {
			return err
		}
		if !exists {
			slog.InfoContext(ctx, "auth.logout.blocked",
				"username", username,
				"dry_run", true,
				"reason", "session_not_found",
			)
			return ErrUnauthorized
		}
		slog.InfoContext(ctx, "auth.logout.dry_run_succeeded",
			"username", username,
			"dry_run", true,
		)
		_ = u.writeAudit(ctx, actor.Actor{ID: username, Kind: actor.KindUser}, "auth.logout", true, map[string]any{
			"username": username,
			"mode":     resolveMutationMode(true),
			"dry_run":  true,
		})
		return nil
	}

	if err := u.sessions.Delete(ctx, username, token); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			slog.InfoContext(ctx, "auth.logout.blocked",
				"username", username,
				"dry_run", false,
				"reason", "session_not_found",
			)
			return ErrUnauthorized
		}
		return err
	}

	slog.InfoContext(ctx, "auth.logout.succeeded",
		"username", username,
		"dry_run", false,
	)
	_ = u.writeAudit(ctx, actor.Actor{ID: username, Kind: actor.KindUser}, "auth.logout", true, map[string]any{
		"username": username,
		"mode":     resolveMutationMode(false),
		"dry_run":  false,
	})
	return nil
}

func (u *AuthUseCase) RecordAttempt(ctx context.Context, principal actor.Actor, success bool) error {
	if u.auditLog == nil {
		return nil
	}

	return u.auditLog.Write(ctx, audit.Event{
		Actor:      principal,
		Action:     "auth.attempt",
		Resource:   "session",
		Success:    success,
		OccurredAt: time.Now().UTC(),
	})
}

func (u *AuthUseCase) CanAuthenticate() bool {
	return u.sessions != nil
}

func (u *AuthUseCase) writeAudit(ctx context.Context, principal actor.Actor, action string, success bool, metadata map[string]any) error {
	if u.auditLog == nil {
		return nil
	}
	return u.auditLog.Write(ctx, audit.Event{
		Actor:      principal,
		Action:     action,
		Resource:   "session",
		Success:    success,
		OccurredAt: time.Now().UTC(),
		Metadata:   metadata,
	})
}

func normalizeLoginUser(user domainuser.User) domainuser.User {
	user.Username = strings.TrimSpace(user.Username)
	user.Nickname = strings.TrimSpace(user.Nickname)
	if user.Nickname == "" {
		user.Nickname = user.Username
	}
	user.Avatar = resolveLoginAvatar(user.Ext)
	return user
}

func resolveLoginAvatar(extRaw string) string {
	ext := parseExtJSON(extRaw)
	if avatar := strings.TrimSpace(stringFromAny(ext["avatar"])); avatar != "" {
		return avatar
	}
	return ""
}

func parseExtJSON(raw string) map[string]any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return map[string]any{}
	}
	if parsed == nil {
		return map[string]any{}
	}
	return parsed
}

func stringFromAny(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
