package repository

import (
	domainauth "omniflow-go/internal/domain/auth"
	sessionredis "omniflow-go/internal/repository/redis/session"

	"github.com/redis/go-redis/v9"
)

type SessionRepository = sessionredis.SessionRepository

func NewSessionRepository(client *redis.Client) domainauth.SessionStore {
	return sessionredis.NewSessionRepository(client)
}
