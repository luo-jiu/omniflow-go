package usecase

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"omniflow-go/internal/actor"

	"github.com/redis/go-redis/v9"
)

var (
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrNotFound           = errors.New("resource not found")
	ErrConflict           = errors.New("resource conflict")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnsupportedStorage = errors.New("unsupported object storage implementation")
)

var sharedRedisClient atomic.Pointer[redis.Client]

func setSharedRedisClient(client *redis.Client) {
	if client != nil {
		sharedRedisClient.Store(client)
	}
}

func getSharedRedisClient() *redis.Client {
	return sharedRedisClient.Load()
}

func actorIDToUint64(principal actor.Actor) (uint64, error) {
	id := strings.TrimSpace(principal.ID)
	if id == "" {
		return 0, fmt.Errorf("%w: actor id is required", ErrInvalidArgument)
	}

	parsed, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: actor id must be numeric", ErrInvalidArgument)
	}
	return parsed, nil
}
