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
	"omniflow-go/internal/actor"
	"omniflow-go/internal/authz"
	"omniflow-go/internal/config"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/repository"
	"omniflow-go/internal/storage"
)

type copyTestStore struct {
	storage.ObjectStorage
	uploads, deletes, stats int
	key                     string
	content                 string
	fail                    bool
	statErr                 error
	statSize                *int64
}

func (s *copyTestStore) Bucket() string { return "test" }
func (s *copyTestStore) StatObject(context.Context, string) (storage.ObjectInfo, error) {
	s.stats++
	if s.statErr != nil {
		return storage.ObjectInfo{}, s.statErr
	}
	if s.statSize != nil {
		return storage.ObjectInfo{Size: *s.statSize}, nil
	}
	return storage.ObjectInfo{Size: 5}, nil
}
func (s *copyTestStore) GetObject(context.Context, string) (io.ReadCloser, storage.ObjectInfo, error) {
	return io.NopCloser(strings.NewReader("hello")), storage.ObjectInfo{Size: 5}, nil
}
func (s *copyTestStore) Upload(_ context.Context, key string, r io.Reader, size int64, _ string) error {
	s.uploads++
	s.key = key
	b, err := io.ReadAll(r)
	s.content = string(b)
	if s.fail {
		return errors.New("upload failed")
	}
	return err
}
func (s *copyTestStore) Delete(context.Context, string) error { s.deletes++; return nil }

func newCopyTest(t *testing.T) (*NodeUseCase, sqlmock.Sqlmock, *copyTestStore) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	store := &copyTestStore{}
	registry := storage.NewStorageRegistry()
	_, err = registry.Reload(&config.StorageConfig{DefaultProvider: "test", Providers: map[string]config.ProviderConfig{"test": {Type: "MINIO", Endpoint: "localhost:9000", Bucket: "test"}}}, func(string, config.ProviderConfig) (storage.ObjectStorage, func(), error) { return store, nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	return NewNodeUseCase(repository.NewNodeRepository(gdb), repository.NewTransactor(gdb), nil, nil, registry), mock, store
}

func copyNodeRows(id, parent int, file bool) *sqlmock.Rows {
	kind := 0
	if file {
		kind = 1
	}
	return sqlmock.NewRows([]string{"id", "parent_id", "library_id", "node_type", "name", "ext"}).AddRow(id, parent, 3, kind, "sample", "txt")
}
func expectCopyView(mock sqlmock.Sqlmock, id, parent int, file bool) {
	expectCopyViewSize(mock, id, parent, file, 5)
}
func expectCopyViewSize(mock sqlmock.Sqlmock, id, parent int, file bool, size int64) {
	for i := 0; i < 2; i++ {
		mock.ExpectQuery(`SELECT .* FROM "nodes"`).WillReturnRows(copyNodeRows(id, parent, file))
	}
	if file {
		mock.ExpectQuery(`SELECT .* FROM "node_files"`).WillReturnRows(sqlmock.NewRows([]string{"file_id", "storage_object_id", "file_size", "mime_type"}).AddRow(id, 5, size, "text/plain"))
		mock.ExpectQuery(`SELECT .* FROM "storage_objects"`).WillReturnRows(sqlmock.NewRows([]string{"id", "provider", "bucket", "object_key"}).AddRow(5, "test", "test", "original"))
	}
}
func expectCopyPlan(mock sqlmock.Sqlmock) {
	expectCopyView(mock, 9, 2, true)
	expectCopyView(mock, 10, 2, false)
	mock.ExpectQuery(`WITH RECURSIVE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10).AddRow(2))
}
func expectCopyCreate(mock sqlmock.Sqlmock, conflict bool) {
	expectCopyCreateAt(mock, conflict, 20, 10, true)
}

func expectCopyCreateAt(mock sqlmock.Sqlmock, conflict bool, id, parent int, file bool) {
	expectCopyView(mock, parent, 2, false)
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .* FROM "nodes"`).WillReturnRows(copyNodeRows(parent, 2, false))
	mock.ExpectQuery(`SELECT .* FROM "nodes"`).WillReturnRows(sqlmock.NewRows([]string{"id", "sort_order"}))
	count := 0
	if conflict {
		count = 1
	}
	mock.ExpectQuery(`SELECT count\(\*\) FROM "nodes"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
	if conflict {
		return
	}
	mock.ExpectQuery(`INSERT INTO "nodes"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
	if file {
		mock.ExpectQuery(`INSERT INTO "storage_objects"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(6))
		mock.ExpectQuery(`INSERT INTO "node_files"`).WillReturnRows(sqlmock.NewRows([]string{"file_id"}).AddRow(id))
	}
	expectCopyView(mock, id, parent, file)
}

func TestCopyRecursiveDirectory(t *testing.T) {
	for _, dry := range []bool{true, false} {
		u, mock, store := newCopyTest(t)
		mock.ExpectBegin()
		expectCopyView(mock, 9, 2, false)
		expectCopyView(mock, 10, 2, false)
		mock.ExpectQuery(`WITH RECURSIVE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
		// 单层枚举与元数据装配分别读取节点，实际子节点父 ID 为源目录。
		expectCopyView(mock, 11, 9, true)
		expectCopyCreateAt(mock, false, 20, 10, false)
		expectCopyCreateAt(mock, false, 21, 20, true)
		if dry {
			mock.ExpectRollback()
		} else {
			mock.ExpectCommit()
		}
		out, err := u.Copy(context.Background(), CopyNodeCommand{NodeID: 9, ParentID: 10, LibraryID: 3, Recursive: true, DryRun: dry})
		if err != nil {
			t.Fatal(err)
		}
		if out.CopiedCount != 2 || out.Node.ParentID != 10 || len(out.AffectedParentIDs) != 1 || out.AffectedParentIDs[0] != 10 {
			t.Fatal(out)
		}
		want := 1
		if dry {
			want = 0
		}
		if store.uploads != want {
			t.Fatal(store)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCopyTransactionAndObjects(t *testing.T) {
	for _, scenario := range []string{"execute", "dry-run", "conflict", "upload-failure", "commit-unknown"} {
		t.Run(scenario, func(t *testing.T) {
			u, mock, store := newCopyTest(t)
			mock.ExpectBegin()
			expectCopyPlan(mock)
			expectCopyCreate(mock, scenario == "conflict")
			if scenario == "execute" {
				mock.ExpectCommit()
			} else if scenario == "commit-unknown" {
				mock.ExpectCommit().WillReturnError(errors.New("lost commit acknowledgement"))
			} else {
				mock.ExpectRollback()
			}
			store.fail = scenario == "upload-failure"
			result, err := u.Copy(context.Background(), CopyNodeCommand{NodeID: 9, LibraryID: 3, ParentID: 10, DryRun: scenario == "dry-run"})
			if scenario == "execute" || scenario == "dry-run" {
				if err != nil {
					t.Fatal(err)
				}
				if result.CopiedCount != 1 {
					t.Fatal(result)
				}
				if scenario == "dry-run" && (result.Node.ID != 0 || result.Node.StorageKey != "") {
					t.Fatal(result)
				}
			} else if err == nil {
				t.Fatal("expected failure")
			}
			if scenario == "commit-unknown" && !strings.Contains(err.Error(), "outcome unknown") {
				t.Fatal(err)
			}
			wantUploads := 1
			if scenario == "dry-run" || scenario == "conflict" {
				wantUploads = 0
			}
			if store.uploads != wantUploads || store.stats != 1 {
				t.Fatalf("store=%+v", store)
			}
			wantDeletes := 0
			if scenario == "upload-failure" {
				wantDeletes = 1
			}
			if store.deletes != wantDeletes {
				t.Fatalf("unsafe cleanup: %+v", store)
			}
			if store.uploads > 0 && (store.key == "original" || store.content != "hello") {
				t.Fatalf("non-independent copy: %+v", store)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCopyRejectRootAndImplicitDirectory(t *testing.T) {
	for _, parent := range []int{0, 2} {
		u, mock, store := newCopyTest(t)
		mock.ExpectBegin()
		expectCopyView(mock, 9, parent, false)
		mock.ExpectRollback()
		_, err := u.Copy(context.Background(), CopyNodeCommand{NodeID: 9, LibraryID: 3, ParentID: 10})
		if !errors.Is(err, ErrInvalidArgument) || store.uploads != 0 {
			t.Fatalf("err=%v store=%+v", err, store)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCopyCompleteFilename(t *testing.T) {
	for _, full := range []string{"report.xlsx", "archive.tar.gz", "README", ".env"} {
		name, ext, err := copyNodeName(full, domainnode.TypeFile)
		if err != nil || nodeVisibleName(name, ext, domainnode.TypeFile) != full {
			t.Fatalf("%s -> %s %s: %v", full, name, ext, err)
		}
	}
	for _, full := range []string{"../x", "a/b", "..", " ", "a\x00b"} {
		if _, _, err := copyNodeName(full, domainnode.TypeFile); err == nil {
			t.Fatalf("accepted %q", full)
		}
	}
}

type copyDenyAuthorizer struct{}

func (copyDenyAuthorizer) Authorize(context.Context, actor.Actor, authz.Resource, authz.Action) error {
	return authz.ErrPermissionDenied
}

func TestCopyPermissionDeniedBeforeTransaction(t *testing.T) {
	u, mock, store := newCopyTest(t)
	u.authorizer = copyDenyAuthorizer{}
	_, err := u.Copy(context.Background(), CopyNodeCommand{NodeID: 9, LibraryID: 3, ParentID: 10})
	if !errors.Is(err, authz.ErrPermissionDenied) || store.stats != 0 || store.uploads != 0 {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCopyByteLimitBeforeWrites(t *testing.T) {
	u, mock, store := newCopyTest(t)
	size := maxCopyBytes + 1
	store.statSize = &size
	mock.ExpectBegin()
	expectCopyViewSize(mock, 9, 2, true, size)
	expectCopyView(mock, 10, 2, false)
	mock.ExpectQuery(`WITH RECURSIVE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
	mock.ExpectRollback()
	_, err := u.Copy(context.Background(), CopyNodeCommand{NodeID: 9, LibraryID: 3, ParentID: 10, DryRun: true})
	if !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), "1 GiB") || store.uploads != 0 {
		t.Fatalf("%v %+v", err, store)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCopySourceAndTreeValidation(t *testing.T) {
	for _, scenario := range []string{"offline", "changed-size", "subtree", "missing-target", "node-limit"} {
		t.Run(scenario, func(t *testing.T) {
			u, mock, store := newCopyTest(t)
			mock.ExpectBegin()
			isFile := scenario != "node-limit"
			expectCopyView(mock, 9, 2, isFile)
			if scenario == "missing-target" {
				mock.ExpectQuery(`SELECT .* FROM "nodes"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			} else {
				expectCopyView(mock, 10, 2, false)
				rows := sqlmock.NewRows([]string{"id"}).AddRow(10)
				if scenario == "subtree" {
					rows.AddRow(9)
				}
				mock.ExpectQuery(`WITH RECURSIVE`).WillReturnRows(rows)
				if scenario == "node-limit" {
					rows := sqlmock.NewRows([]string{"id", "parent_id", "library_id", "node_type", "name"})
					for i := 0; i < 1000; i++ {
						rows.AddRow(100+i, 9, 3, 0, "child")
					}
					mock.ExpectQuery(`SELECT .* FROM "nodes".*LIMIT`).WillReturnRows(rows)
					loaded := sqlmock.NewRows([]string{"id", "parent_id", "library_id", "node_type", "name"})
					for i := 0; i < 1000; i++ {
						loaded.AddRow(100+i, 9, 3, 0, "child")
					}
					mock.ExpectQuery(`SELECT .* FROM "nodes"`).WillReturnRows(loaded)
				}
			}
			if scenario == "offline" {
				store.statErr = errors.New("offline")
			}
			if scenario == "changed-size" {
				size := int64(6)
				store.statSize = &size
			}
			mock.ExpectRollback()
			_, err := u.Copy(context.Background(), CopyNodeCommand{NodeID: 9, LibraryID: 3, ParentID: 10, Recursive: true, DryRun: true})
			if err == nil || store.uploads != 0 {
				t.Fatalf("err=%v uploads=%d", err, store.uploads)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
