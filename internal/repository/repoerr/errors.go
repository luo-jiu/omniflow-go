package repoerr

import "errors"

var (
	ErrNotFound     = errors.New("repository: resource not found")
	ErrConflict     = errors.New("repository: resource conflict")
	ErrInvalidState = errors.New("repository: invalid resource state")
)
