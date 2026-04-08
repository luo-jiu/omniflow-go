package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	domainuser "omniflow-go/internal/domain/user"
	"omniflow-go/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	loginRedisPrefix = "login:"
	loginTTL         = 30 * 24 * time.Hour
)

type LoginCommand struct {
	Actor    actor.Actor
	Username string
	Password string
}

type LoginResult struct {
	Token string
}

type AuthUseCase struct {
	users    *repository.UserRepository
	auditLog audit.Sink
	redis    *redis.Client
}

func NewAuthUseCase(users *repository.UserRepository, auditLog audit.Sink, redisClient ...*redis.Client) *AuthUseCase {
	var client *redis.Client
	if len(redisClient) > 0 {
		client = redisClient[0]
		setSharedRedisClient(client)
	}

	return &AuthUseCase{
		users:    users,
		auditLog: auditLog,
		redis:    client,
	}
}

func (u *AuthUseCase) Login(ctx context.Context, cmd LoginCommand) (LoginResult, error) {
	username := strings.TrimSpace(cmd.Username)
	password := strings.TrimSpace(cmd.Password)
	if username == "" || password == "" {
		return LoginResult{}, fmt.Errorf("%w: username and password are required", ErrInvalidArgument)
	}

	db, err := dbFromRepository(u.users)
	if err != nil {
		return LoginResult{}, err
	}

	client := u.redisClient()
	if client == nil {
		return LoginResult{}, fmt.Errorf("%w: redis client not configured", ErrInvalidArgument)
	}

	var user userRecord
	if err := db.WithContext(ctx).
		Where("username = ? AND status = ?", username, userStatusActive).
		First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = u.RecordAttempt(ctx, cmd.Actor, false)
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		_ = u.RecordAttempt(ctx, cmd.Actor, false)
		return LoginResult{}, ErrInvalidCredentials
	}

	key := loginRedisPrefix + username
	if token, err := u.firstActiveToken(ctx, client, key); err == nil && token != "" {
		_ = u.RecordAttempt(ctx, cmd.Actor, true)
		_ = u.writeAudit(ctx, cmd.Actor, "auth.login", true, map[string]any{
			"username": username,
			"token":    token,
			"cached":   true,
		})
		return LoginResult{Token: token}, nil
	}

	token := uuid.NewString()
	userPayload, err := json.Marshal(user.toDomain())
	if err != nil {
		return LoginResult{}, err
	}

	if err := client.HSet(ctx, key, token, userPayload).Err(); err != nil {
		return LoginResult{}, err
	}
	if err := client.Expire(ctx, key, loginTTL).Err(); err != nil {
		return LoginResult{}, err
	}

	_ = u.RecordAttempt(ctx, cmd.Actor, true)
	_ = u.writeAudit(ctx, cmd.Actor, "auth.login", true, map[string]any{
		"username": username,
		"token":    token,
		"cached":   false,
	})
	return LoginResult{Token: token}, nil
}

func (u *AuthUseCase) Check(ctx context.Context, username, token string) (bool, error) {
	username = strings.TrimSpace(username)
	token = strings.TrimSpace(token)
	if username == "" || token == "" {
		return false, fmt.Errorf("%w: username and token are required", ErrInvalidArgument)
	}

	client := u.redisClient()
	if client == nil {
		return false, fmt.Errorf("%w: redis client not configured", ErrInvalidArgument)
	}

	ok, err := client.HExists(ctx, loginRedisPrefix+username, token).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (u *AuthUseCase) ResolveActor(ctx context.Context, username, token string) (actor.Actor, error) {
	username = strings.TrimSpace(username)
	token = strings.TrimSpace(token)
	if username == "" || token == "" {
		return actor.Actor{}, fmt.Errorf("%w: username and token are required", ErrInvalidArgument)
	}

	client := u.redisClient()
	if client == nil {
		return actor.Actor{}, fmt.Errorf("%w: redis client not configured", ErrInvalidArgument)
	}

	payload, err := client.HGet(ctx, loginRedisPrefix+username, token).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return actor.Actor{}, ErrUnauthorized
		}
		return actor.Actor{}, err
	}

	var sessionUser domainuser.User
	if err := json.Unmarshal([]byte(payload), &sessionUser); err != nil {
		return actor.Actor{}, err
	}

	if sessionUser.ID == 0 {
		db, err := dbFromRepository(u.users)
		if err != nil {
			return actor.Actor{}, err
		}

		var found userRecord
		if err := db.WithContext(ctx).
			Where("username = ? AND status = ?", username, userStatusActive).
			First(&found).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return actor.Actor{}, ErrUnauthorized
			}
			return actor.Actor{}, err
		}
		sessionUser = found.toDomain()
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

func (u *AuthUseCase) Logout(ctx context.Context, username, token string) error {
	ok, err := u.Check(ctx, username, token)
	if err != nil {
		return err
	}
	if !ok {
		return ErrUnauthorized
	}

	client := u.redisClient()
	if err := client.Del(ctx, loginRedisPrefix+strings.TrimSpace(username)).Err(); err != nil {
		return err
	}

	_ = u.writeAudit(ctx, actor.Actor{ID: username, Kind: actor.KindUser}, "auth.logout", true, map[string]any{
		"username": strings.TrimSpace(username),
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

func (u *AuthUseCase) firstActiveToken(ctx context.Context, client *redis.Client, key string) (string, error) {
	exists, err := client.Exists(ctx, key).Result()
	if err != nil || exists == 0 {
		return "", err
	}
	entries, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		return "", err
	}
	for token := range entries {
		return token, nil
	}
	return "", nil
}

func (u *AuthUseCase) redisClient() *redis.Client {
	if u.redis != nil {
		return u.redis
	}
	return getSharedRedisClient()
}

func (u *AuthUseCase) CanAuthenticate() bool {
	return u.redisClient() != nil
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
