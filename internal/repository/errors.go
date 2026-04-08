package repository

import "omniflow-go/internal/repository/repoerr"

var (
	ErrNotFound     = repoerr.ErrNotFound
	ErrConflict     = repoerr.ErrConflict
	ErrInvalidState = repoerr.ErrInvalidState
)
