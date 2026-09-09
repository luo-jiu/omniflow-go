package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"omniflow-go/internal/repository/postgres/impl/txctx"
)

func TestReplaceFileStorageConditional(t *testing.T) {
	for _, scenario := range []string{"match", "stale", "missing-binding", "update-failure"} {
		t.Run(scenario, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
				DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent),
			})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT .* FROM "nodes" .*FOR UPDATE`).
				WillReturnRows(sqlmock.NewRows([]string{"id", "library_id", "node_type"}).AddRow(9, 3, 1))
			binding := sqlmock.NewRows([]string{"file_id", "library_id", "storage_object_id"})
			if scenario != "missing-binding" {
				binding.AddRow(9, 3, 7)
			}
			mock.ExpectQuery(`SELECT .* FROM "node_files"`).WillReturnRows(binding)
			if scenario != "missing-binding" {
				mock.ExpectQuery(`SELECT .* FROM "storage_objects"`).
					WillReturnRows(sqlmock.NewRows([]string{"id", "object_key"}).AddRow(7, "old"))
			}
			expected := "old"
			if scenario == "stale" {
				expected = "stale"
			}
			if scenario == "match" || scenario == "update-failure" {
				mock.ExpectExec(`UPDATE "storage_objects"`).WillReturnResult(sqlmock.NewResult(0, 1))
				if scenario == "update-failure" {
					mock.ExpectExec(`UPDATE "node_files"`).WillReturnError(errors.New("database unavailable"))
				} else {
					mock.ExpectExec(`UPDATE "node_files"`).WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectExec(`UPDATE "nodes"`).WillReturnResult(sqlmock.NewResult(0, 1))
				}
			}
			if scenario == "match" {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			var oldKey string
			err = txctx.NewGormTransactor(db).WithinTx(context.Background(), func(ctx context.Context) error {
				var writeErr error
				oldKey, writeErr = NewNodeRepository(db).ReplaceFileStorage(ctx, 9, 3, ReplaceFileStorageInput{
					ExpectedStorageKey: &expected, NewObjectKey: "new", NewFileSize: 3,
					NewContentType: "text/plain", NewProvider: "local", NewBucket: "test",
				})
				return writeErr
			})
			if scenario == "match" {
				if err != nil || oldKey != "old" {
					t.Fatalf("key=%q err=%v", oldKey, err)
				}
			} else if err == nil {
				t.Fatal("expected write to fail")
			}
			if (scenario == "stale" || scenario == "missing-binding") && !errors.Is(err, ErrConflict) {
				t.Fatalf("expected conflict, got %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
