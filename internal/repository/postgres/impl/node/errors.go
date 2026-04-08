package repository

import (
	"errors"

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
	return err
}
