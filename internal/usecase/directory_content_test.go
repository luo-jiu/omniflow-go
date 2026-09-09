package usecase

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"omniflow-go/internal/config"
	"omniflow-go/internal/repository"
	"omniflow-go/internal/storage"
)

type contentTestStorage struct {
	storage.ObjectStorage
	uploads int
}

func (s *contentTestStorage) Upload(context.Context, string, io.Reader, int64, string) error {
	s.uploads++
	return errors.New("test storage offline")
}

func TestUpdateFileContentDryRunAndPrecondition(t *testing.T) {
	for _, scenario := range []string{"dry-run", "execute", "stale-dry-run", "stale-execute"} {
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
			for i := 0; i < 2; i++ {
				mock.ExpectQuery(`SELECT .* FROM "nodes"`).WillReturnRows(
					sqlmock.NewRows([]string{"id", "library_id", "node_type", "name"}).AddRow(9, 3, 1, "test"))
			}
			mock.ExpectQuery(`SELECT .* FROM "node_files"`).WillReturnRows(sqlmock.NewRows([]string{"file_id"}))
			store := &contentTestStorage{}
			registry := storage.NewStorageRegistry()
			_, err = registry.Reload(&config.StorageConfig{
				DefaultProvider: "test", Providers: map[string]config.ProviderConfig{
					"test": {Type: "MINIO", Endpoint: "localhost:9000", Bucket: "test"},
				},
			}, func(string, config.ProviderConfig) (storage.ObjectStorage, func(), error) { return store, nil, nil })
			if err != nil {
				t.Fatal(err)
			}
			u := &DirectoryUseCase{nodes: &NodeUseCase{nodes: repository.NewNodeRepository(db)}, registry: registry}
			expected := ""
			stale := strings.HasPrefix(scenario, "stale")
			if stale {
				expected = "old"
			}
			dryRun := strings.HasSuffix(scenario, "dry-run")
			_, err = u.UpdateFileContent(context.Background(), UpdateFileContentCommand{
				LibraryID: 3, NodeID: 9, Content: strings.NewReader("hello"), FileSize: 5,
				ExpectedStorageKey: &expected, DryRun: dryRun,
			})
			if stale {
				if !errors.Is(err, repository.ErrConflict) {
					t.Fatalf("expected conflict, got %v", err)
				}
			} else if dryRun {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "test storage offline") {
				t.Fatalf("expected storage failure: %v", err)
			}
			wanted := 0
			if !stale && !dryRun {
				wanted = 1
			}
			if store.uploads != wanted {
				t.Fatalf("uploads=%d, want %d", store.uploads, wanted)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
