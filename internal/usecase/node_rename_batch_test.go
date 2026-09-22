package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"omniflow-go/internal/actor"
	"omniflow-go/internal/authz"
	domainnode "omniflow-go/internal/domain/node"
)

const renameTestOperation = "12345678-1234-4234-8234-123456789abc"

var renameTestTime = time.Date(2026, 9, 22, 1, 2, 3, 123456000, time.UTC)

func renameTestCommand() RenameBatchCommand {
	return RenameBatchCommand{Actor: actor.Actor{ID: "7", Kind: actor.KindUser}, LibraryID: 3, OperationID: renameTestOperation,
		Items: []domainnode.RenameBatchItem{{NodeID: 9, Name: "new", Expected: domainnode.RenameExpected{
			Name: "old", Ext: "jpg", ParentID: 8, UpdatedAt: renameTestTime,
		}}}}
}

func renameRows(name string, updatedAt time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "library_id", "parent_id", "node_type", "name", "ext", "archive_mode", "updated_at"}).
		AddRow(9, 3, 8, 1, name, "jpg", false, updatedAt)
}

func expectRenameReceiptLookup(mock sqlmock.Sqlmock, receipt *domainnode.MutationReceipt) {
	rows := sqlmock.NewRows([]string{"actor_id", "operation_id", "library_id", "kind", "request_hash", "result"})
	if receipt != nil {
		rows.AddRow(receipt.ActorID, receipt.OperationID, receipt.LibraryID, receipt.Kind, receipt.RequestHash, receipt.Result)
	}
	mock.ExpectQuery(`SELECT .* FROM "node_mutation_receipts"`).WithArgs("user:7", renameTestOperation, 1).WillReturnRows(rows)
}

func expectRenameStart(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs("nodes:mutation:user:7:" + renameTestOperation).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectRenameReceiptLookup(mock, nil)
}

func TestRenameBatchTransaction(t *testing.T) {
	for _, scenario := range []string{"execute", "dry-run", "stale", "missing", "name-conflict", "unique-race", "receipt-failure", "commit-unknown"} {
		t.Run(scenario, func(t *testing.T) {
			u, mock := newTagDeltaTest(t)
			cmd := renameTestCommand()
			cmd.DryRun = scenario == "dry-run"
			expectRenameStart(mock)
			rows := renameRows("old", renameTestTime)
			if scenario == "stale" {
				rows = renameRows("old", renameTestTime.Add(time.Microsecond))
			} else if scenario == "missing" {
				rows = sqlmock.NewRows([]string{"id"})
			}
			mock.ExpectQuery(`SELECT .* FROM "nodes" .*ORDER BY.*FOR UPDATE`).WithArgs(int64(3), int64(9)).WillReturnRows(rows)
			if scenario != "stale" && scenario != "missing" {
				count := 0
				if scenario == "name-conflict" {
					count = 1
				}
				mock.ExpectQuery(`SELECT count\(\*\) FROM "nodes"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
				if count == 0 {
					write := mock.ExpectExec(`UPDATE "nodes" SET`).WithArgs("new", sqlmock.AnyArg(), int64(3), int64(9))
					if scenario == "unique-race" {
						write.WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "uq_nodes_live_sibling_visible_name"})
					} else {
						write.WillReturnResult(sqlmock.NewResult(0, 1))
						mock.ExpectQuery(`SELECT .* FROM "nodes"`).WithArgs(int64(9), int64(3), 1).
							WillReturnRows(renameRows("new", renameTestTime.Add(time.Second)))
						save := mock.ExpectQuery(`INSERT INTO "node_mutation_receipts"`)
						if scenario == "receipt-failure" {
							save.WillReturnError(errors.New("receipt unavailable"))
						} else {
							save.WillReturnRows(sqlmock.NewRows([]string{"committed_at"}).AddRow(renameTestTime.Add(time.Second)))
						}
					}
				}
			}
			switch scenario {
			case "execute":
				mock.ExpectCommit()
			case "commit-unknown":
				mock.ExpectCommit().WillReturnError(errors.New("connection lost"))
			default:
				mock.ExpectRollback()
			}
			out, err := u.RenameBatch(context.Background(), cmd)
			switch scenario {
			case "execute", "dry-run":
				if err != nil || len(out.Items) != 1 || !out.Atomic || out.Replayed || out.Items[0].Current.Ext != "jpg" {
					t.Fatalf("%+v %v", out, err)
				}
				wantTime, wantState := renameTestTime.Add(time.Second), "committed"
				if cmd.DryRun {
					wantTime, wantState = renameTestTime, "validated"
				}
				if out.State != wantState || out.Items[0].Status != wantState || !out.Items[0].Current.UpdatedAt.Equal(wantTime) {
					t.Fatalf("%+v", out)
				}
			case "stale", "name-conflict", "unique-race":
				if !errors.Is(err, ErrConflict) {
					t.Fatalf("expected conflict: %v", err)
				}
			case "missing":
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("expected not found: %v", err)
				}
			case "commit-unknown":
				if err == nil || !strings.Contains(err.Error(), "outcome unknown") || !strings.Contains(err.Error(), renameTestOperation) {
					t.Fatalf("%v", err)
				}
			default:
				if err == nil {
					t.Fatal("expected rollback")
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRenameBatchReplayAndStatus(t *testing.T) {
	for _, scenario := range []string{"replay", "hash-conflict", "dry-run-after-commit", "status", "status-missing", "status-other-library"} {
		t.Run(scenario, func(t *testing.T) {
			u, mock := newTagDeltaTest(t)
			cmd := renameTestCommand()
			_, hash, err := validateRenameBatch(cmd)
			if err != nil {
				t.Fatal(err)
			}
			result := domainnode.RenameBatchResult{OperationID: cmd.OperationID, LibraryID: 3, Atomic: true, State: "committed", Items: []domainnode.RenameItemResult{}}
			encoded, _ := json.Marshal(result)
			receipt := &domainnode.MutationReceipt{ActorID: "user:7", OperationID: cmd.OperationID, LibraryID: 3,
				Kind: "rename_batch", RequestHash: hash, Result: string(encoded)}
			if strings.HasPrefix(scenario, "status") {
				if scenario == "status-missing" {
					receipt = nil
				} else if scenario == "status-other-library" {
					receipt.LibraryID = 4
				}
				expectRenameReceiptLookup(mock, receipt)
				out, err := u.RenameBatchStatus(context.Background(), cmd.Actor, cmd.LibraryID, cmd.OperationID)
				if err != nil || (scenario == "status" && (out.State != "committed" || out.Result == nil)) ||
					(scenario != "status" && (out.State != "not_found" || out.Result != nil)) {
					t.Fatalf("%+v %v", out, err)
				}
			} else {
				mock.ExpectBegin()
				mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
				if scenario == "hash-conflict" {
					receipt.RequestHash = strings.Repeat("0", 64)
				}
				cmd.DryRun = scenario == "dry-run-after-commit"
				expectRenameReceiptLookup(mock, receipt)
				if scenario == "replay" {
					mock.ExpectCommit()
				} else {
					mock.ExpectRollback()
				}
				out, err := u.RenameBatch(context.Background(), cmd)
				if scenario == "replay" {
					if err != nil || !out.Replayed || out.State != "committed" {
						t.Fatalf("%+v %v", out, err)
					}
				} else if !errors.Is(err, ErrConflict) {
					t.Fatalf("%v", err)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRenameBatchValidationAndAuthorization(t *testing.T) {
	for _, modify := range []func(*RenameBatchCommand){
		func(c *RenameBatchCommand) { c.Items = nil },
		func(c *RenameBatchCommand) { c.Items = append(c.Items, c.Items[0]) },
		func(c *RenameBatchCommand) { c.OperationID = strings.ToUpper(c.OperationID) },
		func(c *RenameBatchCommand) { c.Items[0].Expected.UpdatedAt = time.Time{} },
		func(c *RenameBatchCommand) { c.Items[0].Expected.ParentID = 0 },
		func(c *RenameBatchCommand) { c.Items[0].Name = "../escape" },
		func(c *RenameBatchCommand) { c.Items[0].Name = "bad\x00name" },
		func(c *RenameBatchCommand) { c.Items[0].Name = "bad\nname" },
		func(c *RenameBatchCommand) { c.Items[0].Name = strings.Repeat("字", 256) },
	} {
		cmd := renameTestCommand()
		modify(&cmd)
		if _, _, err := validateRenameBatch(cmd); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("accepted %+v: %v", cmd, err)
		}
	}
	u, mock := newTagDeltaTest(t)
	u.authorizer = tagDeltaDenyAuthorizer{}
	cmd := renameTestCommand()
	if _, err := u.RenameBatch(context.Background(), cmd); !errors.Is(err, authz.ErrPermissionDenied) {
		t.Fatal(err)
	}
	if _, err := u.RenameBatchStatus(context.Background(), cmd.Actor, 3, cmd.OperationID); !errors.Is(err, authz.ErrPermissionDenied) {
		t.Fatal(err)
	}
	cmd.Actor = actor.Anonymous()
	if _, err := u.RenameBatch(context.Background(), cmd); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRenameBatchHashCanonicalizesOrderAndTimeZone(t *testing.T) {
	cmd := renameTestCommand()
	other := cmd.Items[0]
	other.NodeID = 10
	cmd.Items = append(cmd.Items, other)
	_, first, _ := validateRenameBatch(cmd)
	cmd.Items[0], cmd.Items[1] = cmd.Items[1], cmd.Items[0]
	cmd.Items[0].Expected.UpdatedAt = cmd.Items[0].Expected.UpdatedAt.In(time.FixedZone("local", 8*3600))
	_, second, _ := validateRenameBatch(cmd)
	if first != second {
		t.Fatal("equivalent snapshots produced different identities")
	}
	cmd.Items[0].Expected.UpdatedAt = cmd.Items[0].Expected.UpdatedAt.Add(time.Nanosecond)
	_, second, _ = validateRenameBatch(cmd)
	if first == second {
		t.Fatal("version precision was discarded")
	}
}

func TestRenameBatchRollsBackAllItemsWhenSecondWriteFails(t *testing.T) {
	u, mock := newTagDeltaTest(t)
	cmd := renameTestCommand()
	second := cmd.Items[0]
	second.NodeID, second.Expected.Name, second.Name = 10, "second", "next"
	cmd.Items = append(cmd.Items, second)
	expectRenameStart(mock)
	rows := renameRows("old", renameTestTime).AddRow(10, 3, 8, 1, "second", "jpg", false, renameTestTime)
	mock.ExpectQuery(`SELECT .* FROM "nodes" .*FOR UPDATE`).WithArgs(int64(3), int64(9), int64(10)).WillReturnRows(rows)
	for range cmd.Items {
		mock.ExpectQuery(`SELECT count\(\*\) FROM "nodes"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	}
	mock.ExpectExec(`UPDATE "nodes" SET`).WithArgs("new", sqlmock.AnyArg(), int64(3), int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .* FROM "nodes"`).WillReturnRows(renameRows("new", renameTestTime.Add(time.Second)))
	mock.ExpectExec(`UPDATE "nodes" SET`).WithArgs("next", sqlmock.AnyArg(), int64(3), int64(10)).WillReturnError(errors.New("write unavailable"))
	mock.ExpectRollback()
	result, err := u.RenameBatch(context.Background(), cmd)
	if err == nil || len(result.Items) != 0 {
		t.Fatalf("partial success escaped atomic rollback: %+v %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRenameBatchRejectsSecondStaleSnapshotBeforeAnyWrite(t *testing.T) {
	u, mock := newTagDeltaTest(t)
	cmd := renameTestCommand()
	second := cmd.Items[0]
	second.NodeID, second.Expected.Name, second.Name = 10, "second", "next"
	cmd.Items = append(cmd.Items, second)
	expectRenameStart(mock)
	mock.ExpectQuery(`SELECT .* FROM "nodes" .*FOR UPDATE`).WillReturnRows(renameRows("old", renameTestTime).
		AddRow(10, 3, 8, 1, "changed", "jpg", false, renameTestTime))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "nodes"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectRollback()
	_, err := u.RenameBatch(context.Background(), cmd)
	if !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRenameBatchRejectsDirectoryAndArchiveMode(t *testing.T) {
	for _, directory := range []bool{true, false} {
		u, mock := newTagDeltaTest(t)
		cmd := renameTestCommand()
		expectRenameStart(mock)
		kind := 1
		if directory {
			kind = 0
		}
		rows := sqlmock.NewRows([]string{"id", "library_id", "parent_id", "node_type", "name", "ext", "archive_mode", "updated_at"}).
			AddRow(9, 3, 8, kind, "old", "jpg", !directory, renameTestTime)
		mock.ExpectQuery(`SELECT .* FROM "nodes" .*FOR UPDATE`).WillReturnRows(rows)
		mock.ExpectRollback()
		if _, err := u.RenameBatch(context.Background(), cmd); !errors.Is(err, ErrInvalidArgument) {
			t.Fatal(err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}
