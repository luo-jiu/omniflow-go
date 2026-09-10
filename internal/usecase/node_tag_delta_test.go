package usecase

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"omniflow-go/internal/actor"
	"omniflow-go/internal/authz"
	"omniflow-go/internal/repository"
)

func newTagDeltaTest(t *testing.T) (*NodeUseCase, sqlmock.Sqlmock) {
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
	return NewNodeUseCase(repository.NewNodeRepository(gdb), repository.NewTransactor(gdb), nil, nil, nil), mock
}

type tagMetadataArgument struct{}

func (tagMetadataArgument) Match(value driver.Value) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	var meta map[string]json.RawMessage
	if json.Unmarshal([]byte(text), &meta) != nil {
		return false
	}
	return string(meta["keep"]) == `{"large":9007199254740993}` && string(meta["tagIds"]) == `[1,99]`
}

func TestNodeTagDeltaTransaction(t *testing.T) {
	for _, mode := range []string{"execute", "dry-run", "foreign-owner", "commit-unknown"} {
		t.Run(mode, func(t *testing.T) {
			u, mock := newTagDeltaTest(t)
			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT .* FROM "nodes" .*FOR UPDATE`).WithArgs(int64(9), int64(3), 1).
				WillReturnRows(sqlmock.NewRows([]string{"id", "library_id", "parent_id", "view_meta"}).AddRow(9, 3, 8, `{"tagIds":[2],"keep":{"large":9007199254740993}}`))
			tags := sqlmock.NewRows([]string{"id", "owner_user_id"})
			if mode != "foreign-owner" {
				tags.AddRow(2, 7)
			}
			mock.ExpectQuery(`SELECT .* FROM "tags" .*FOR SHARE`).WillReturnRows(tags)
			if mode == "foreign-owner" {
				mock.ExpectRollback()
			} else {
				mock.ExpectExec(`DELETE FROM "node_tag_rel" .*"tag_id" =`).WithArgs(int64(9), int64(3), int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(`SELECT .* FROM "node_tag_rel"`).WithArgs(int64(9), int64(3)).
					WillReturnRows(sqlmock.NewRows([]string{"node_id", "library_id", "tag_id"}).AddRow(9, 3, 1).AddRow(9, 3, 99))
				mock.ExpectExec(`UPDATE "nodes" SET`).WithArgs(sqlmock.AnyArg(), tagMetadataArgument{}, int64(9), int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
				if mode == "dry-run" {
					mock.ExpectRollback()
				} else if mode == "commit-unknown" {
					mock.ExpectCommit().WillReturnError(errors.New("connection lost"))
				} else {
					mock.ExpectCommit()
				}
			}
			out, err := u.UpdateNodeTags(context.Background(), NodeTagDeltaCommand{Actor: actor.Actor{ID: "7", Kind: actor.KindUser}, NodeID: 9, LibraryID: 3, RemoveTagIDs: []uint64{2}, DryRun: mode == "dry-run"})
			if mode == "foreign-owner" || mode == "commit-unknown" {
				if err == nil {
					t.Fatal("expected rejection")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if len(out.TagIDs) != 2 || out.TagIDs[1] != 99 || out.DryRun != (mode == "dry-run") {
					t.Fatalf("%+v", out)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNodeTagDeltaValidation(t *testing.T) {
	for _, cmd := range []NodeTagDeltaCommand{{}, {NodeID: 9, LibraryID: 3}, {NodeID: 9, LibraryID: 3, AddTagIDs: []uint64{0}},
		{NodeID: 9, LibraryID: 3, AddTagIDs: []uint64{1}, RemoveTagIDs: []uint64{1}}, {NodeID: 9, LibraryID: 3, AddTagIDs: []uint64{1, 1}}} {
		if validateNodeTagDelta(cmd) == nil {
			t.Fatalf("accepted %+v", cmd)
		}
	}
}

func TestNodeTagDeltaReadAndMissingNode(t *testing.T) {
	for _, missing := range []bool{false, true} {
		u, mock := newTagDeltaTest(t)
		mock.ExpectBegin()
		rows := sqlmock.NewRows([]string{"id", "library_id", "parent_id"})
		if !missing {
			rows.AddRow(9, 3, 8)
		}
		mock.ExpectQuery(`SELECT .* FROM "nodes" .*FOR SHARE`).WithArgs(int64(9), int64(3), 1).WillReturnRows(rows)
		if missing {
			mock.ExpectRollback()
		} else {
			mock.ExpectQuery(`SELECT .* FROM "node_tag_rel"`).WillReturnRows(sqlmock.NewRows([]string{"tag_id"}).AddRow(99))
			mock.ExpectCommit()
		}
		out, err := u.ReadNodeTags(context.Background(), actor.Actor{ID: "7", Kind: actor.KindUser}, 9, 3)
		if missing {
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("%v", err)
			}
		} else if err != nil || len(out.TagIDs) != 1 {
			t.Fatalf("%+v %v", out, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNodeTagDeltaAdditionPolicyFailureRollsBack(t *testing.T) {
	u, mock := newTagDeltaTest(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "nodes" .*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "library_id", "view_meta"}).AddRow(9, 3, `{}`))
	mock.ExpectQuery(`SELECT .* FROM "tags" .*FOR SHARE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tags"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT .* FROM "node_resource_targets"`).WillReturnRows(sqlmock.NewRows([]string{"node_id", "target_kind"}).AddRow(9, "file"))
	mock.ExpectQuery(`SELECT .* FROM "tag_bind_policies"`).WillReturnRows(sqlmock.NewRows([]string{"tag_id", "target_kind"}).AddRow(1, "folder"))
	mock.ExpectRollback()
	_, err := u.UpdateNodeTags(context.Background(), NodeTagDeltaCommand{Actor: actor.Actor{ID: "7", Kind: actor.KindUser}, NodeID: 9, LibraryID: 3, AddTagIDs: []uint64{1}})
	if err == nil || !strings.Contains(err.Error(), "not bindable") {
		t.Fatalf("%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type tagDeltaDenyAuthorizer struct{}

func (tagDeltaDenyAuthorizer) Authorize(context.Context, actor.Actor, authz.Resource, authz.Action) error {
	return authz.ErrPermissionDenied
}

func TestNodeTagDeltaAuthorization(t *testing.T) {
	u, mock := newTagDeltaTest(t)
	u.authorizer = tagDeltaDenyAuthorizer{}
	principal := actor.Actor{ID: "7", Kind: actor.KindUser}
	_, err := u.UpdateNodeTags(context.Background(), NodeTagDeltaCommand{Actor: principal, NodeID: 9, LibraryID: 3, AddTagIDs: []uint64{1}})
	if !errors.Is(err, authz.ErrPermissionDenied) {
		t.Fatal(err)
	}
	_, err = u.ReadNodeTags(context.Background(), principal, 9, 3)
	if !errors.Is(err, authz.ErrPermissionDenied) {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNodeTagDeltaAdditionPreservesExistingRelationships(t *testing.T) {
	u, mock := newTagDeltaTest(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "nodes" .*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "library_id", "view_meta"}).AddRow(9, 3, `{"keep":{"large":9007199254740993}}`))
	mock.ExpectQuery(`SELECT .* FROM "tags" .*FOR SHARE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tags"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT .* FROM "node_resource_targets"`).WillReturnRows(sqlmock.NewRows([]string{"node_id", "target_kind"}).AddRow(9, "file"))
	mock.ExpectQuery(`SELECT .* FROM "tag_bind_policies"`).WillReturnRows(sqlmock.NewRows([]string{"tag_id", "target_kind"}).AddRow(1, "file"))
	mock.ExpectQuery(`INSERT INTO "node_tag_rel" .*ON CONFLICT .*DO NOTHING`).WithArgs(int64(9), int64(1), int64(3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(20))
	mock.ExpectQuery(`SELECT .* FROM "node_tag_rel"`).WillReturnRows(sqlmock.NewRows([]string{"tag_id"}).AddRow(1).AddRow(99))
	mock.ExpectExec(`UPDATE "nodes" SET`).WithArgs(sqlmock.AnyArg(), tagMetadataArgument{}, int64(9), int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	out, err := u.UpdateNodeTags(context.Background(), NodeTagDeltaCommand{Actor: actor.Actor{ID: "7", Kind: actor.KindUser}, NodeID: 9, LibraryID: 3, AddTagIDs: []uint64{1}})
	if err != nil || len(out.TagIDs) != 2 || out.TagIDs[1] != 99 {
		t.Fatalf("%+v %v", out, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
