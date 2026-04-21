package repository

import (
	"errors"

	"omniflow-go/internal/repository/repoerr"

	"github.com/jackc/pgx/v5/pgconn"
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
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		var typed *pgconn.PgError
		if errors.As(err, &typed) && typed.ConstraintName == "uq_nodes_live_sibling_visible_name" {
			return ErrConflict
		}
		return err
	}
	var typed *pgconn.PgError
	if errors.As(err, &typed) && typed.ConstraintName == "uq_nodes_live_sibling_visible_name" {
		return ErrConflict
	}
	return err
}
