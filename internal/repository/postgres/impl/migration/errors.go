package repository

import (
	"errors"

	"omniflow-go/internal/repository/repoerr"

	"gorm.io/gorm"
)

var (
	// ErrNotFound 任务或子项不存在。
	ErrNotFound = repoerr.ErrNotFound
	// ErrConflict 任务状态冲突。
	ErrConflict = repoerr.ErrConflict
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
