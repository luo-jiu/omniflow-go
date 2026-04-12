package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domainuser "omniflow-go/internal/domain/user"
	"omniflow-go/internal/repository/repoerr"

	"github.com/redis/go-redis/v9"
)

const (
	loginRedisPrefix = "login:"
	loginTTL         = 30 * 24 * time.Hour
)

// SessionRepository 管理登录态 token 在 Redis 中的读写。
type SessionRepository struct {
	client *redis.Client
}

func NewSessionRepository(client *redis.Client) *SessionRepository {
	return &SessionRepository{client: client}
}

func (r *SessionRepository) FindFirstToken(ctx context.Context, username string) (string, error) {
	if err := r.ensureClient(); err != nil {
		return "", err
	}

	key := r.loginKey(username)
	entries, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return "", err
	}
	for token := range entries {
		return token, nil
	}
	return "", nil
}

func (r *SessionRepository) Save(ctx context.Context, username, token string, user domainuser.User) error {
	if err := r.ensureClient(); err != nil {
		return err
	}

	payload, err := json.Marshal(user)
	if err != nil {
		return err
	}

	key := r.loginKey(username)
	if err := r.client.HSet(ctx, key, token, payload).Err(); err != nil {
		return err
	}
	if err := r.client.Expire(ctx, key, loginTTL).Err(); err != nil {
		return err
	}
	return nil
}

func (r *SessionRepository) Exists(ctx context.Context, username, token string) (bool, error) {
	if err := r.ensureClient(); err != nil {
		return false, err
	}
	return r.client.HExists(ctx, r.loginKey(username), token).Result()
}

func (r *SessionRepository) GetUser(ctx context.Context, username, token string) (domainuser.User, error) {
	if err := r.ensureClient(); err != nil {
		return domainuser.User{}, err
	}

	payload, err := r.client.HGet(ctx, r.loginKey(username), token).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return domainuser.User{}, repoerr.ErrNotFound
		}
		return domainuser.User{}, err
	}

	var user domainuser.User
	if err := json.Unmarshal([]byte(payload), &user); err != nil {
		return domainuser.User{}, err
	}
	return user, nil
}

func (r *SessionRepository) Delete(ctx context.Context, username, token string) error {
	if err := r.ensureClient(); err != nil {
		return err
	}

	key := r.loginKey(username)
	removed, err := r.client.HDel(ctx, key, token).Result()
	if err != nil {
		return err
	}
	if removed == 0 {
		return repoerr.ErrNotFound
	}

	remainCount, err := r.client.HLen(ctx, key).Result()
	if err != nil {
		return err
	}
	if remainCount == 0 {
		if err := r.client.Del(ctx, key).Err(); err != nil {
			return err
		}
		return nil
	}
	return r.client.Expire(ctx, key, loginTTL).Err()
}

func (r *SessionRepository) loginKey(username string) string {
	return loginRedisPrefix + strings.TrimSpace(username)
}

func (r *SessionRepository) ensureClient() error {
	if r.client == nil {
		return fmt.Errorf("redis session repository: client is nil")
	}
	return nil
}
