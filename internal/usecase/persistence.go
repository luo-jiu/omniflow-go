package usecase

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"omniflow-go/internal/actor"
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
