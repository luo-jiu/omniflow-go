package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"omniflow-go/internal/repository"
)

func TestResolveCreateParentIDStrictDoesNotFallbackToRoot(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	u := NewNodeUseCase(repository.NewNodeRepository(gdb), nil, nil, nil)
	mock.ExpectQuery(`SELECT .* FROM "nodes"`).WithArgs(int64(991), int64(3), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	parent, err := u.resolveCreateParentID(context.Background(), 3, 991, true)
	if !errors.Is(err, ErrNotFound) || parent != 0 {
		t.Fatalf("strict parent must fail closed, parent=%d err=%v", parent, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveCreateParentIDStrictRequiresExplicitParent(t *testing.T) {
	// Ordinary upload callers keep parentId=0 root resolution; strict callers cannot.
	u := &NodeUseCase{}
	if _, err := u.resolveCreateParentID(context.Background(), 3, 0, true); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("strict root fallback must be rejected, got %v", err)
	}
}
