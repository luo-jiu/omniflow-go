package repository

import (
	"errors"
	"strings"

	"omniflow-go/internal/repository/repoerr"

	"gorm.io/gorm"
)

var (
	ErrNotFound     = repoerr.ErrNotFound
	ErrConflict     = repoerr.ErrConflict
	ErrInvalidState = repoerr.ErrInvalidState
)

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
		return ErrConflict
	}
	return err
}
