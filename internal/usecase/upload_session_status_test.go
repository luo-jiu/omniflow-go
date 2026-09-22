package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"omniflow-go/internal/actor"
	"omniflow-go/internal/authz"
	"omniflow-go/internal/repository"
)

type uploadStatusAuthorizer struct {
	err   error
	calls []struct {
		principal actor.Actor
		resource  authz.Resource
		action    authz.Action
	}
}

func (a *uploadStatusAuthorizer) Authorize(_ context.Context, principal actor.Actor, resource authz.Resource, action authz.Action) error {
	a.calls = append(a.calls, struct {
		principal actor.Actor
		resource  authz.Resource
		action    authz.Action
	}{principal, resource, action})
	return a.err
}

func newUploadStatusTest(t *testing.T, authorizer authz.Authorizer) (*UploadSessionUseCase, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{
		DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewUploadSessionUseCase(repository.NewUploadSessionRepository(gdb), nil, nil, nil, authorizer, nil, nil), mock
}

func expectUploadStatusRow(mock sqlmock.Sqlmock, state string, expiresAt time.Time, result string) {
	rows := sqlmock.NewRows([]string{"id", "actor_id", "client_operation_id", "library_id", "status", "expires_at", "completed_node_id", "completion_result"})
	if state != "missing" {
		var nodeID any
		var receipt any
		if result != "" {
			nodeID, receipt = 900, result
		}
		rows.AddRow("upload-fixture", "42", "operation-fixture", 3, state, expiresAt, nodeID, receipt)
	}
	// actor 与 operation 同时作为查询条件；其他 actor 的行不能被返回。
	mock.ExpectQuery(`SELECT .* FROM "upload_sessions" .*"actor_id" = .*"client_operation_id" =`).
		WithArgs("42", "operation-fixture", 1).WillReturnRows(rows)
}

func TestUploadStatusPendingDoesNotClaimDefinitiveFailure(t *testing.T) {
	authorizer := &uploadStatusAuthorizer{}
	u, mock := newUploadStatusTest(t, authorizer)
	principal := actor.Actor{ID: "42", Kind: actor.KindUser}
	expiresAt := time.Now().UTC().Add(time.Hour)
	expectUploadStatusRow(mock, "pending", expiresAt, "")
	first, err := u.ReconcileCompletion(context.Background(), principal, "operation-fixture")
	if err != nil || first.State != UploadCompletionStateUnknown || first.Node != nil {
		t.Fatalf("in-flight completion must remain unknown: %+v %v", first, err)
	}
	// 同一 operation 随后完成，说明第一次 pending 查询不能触发 abort、兜底或新上传。
	expectUploadStatusRow(mock, "committed", expiresAt,
		`{"id":900,"name":"converted","ext":"csv","type":"file","parentId":800,"libraryId":3}`)
	second, err := u.ReconcileCompletion(context.Background(), principal, "operation-fixture")
	if err != nil || second.State != UploadCompletionStateCommitted || second.Node == nil || second.Node.ID != 900 {
		t.Fatalf("completion receipt unavailable: %+v %v", second, err)
	}
	if len(authorizer.calls) != 2 {
		t.Fatalf("expected both lookups to reauthorize, got %d", len(authorizer.calls))
	}
	for _, call := range authorizer.calls {
		if call.principal.ID != "42" || call.resource.Kind != "library" || call.resource.ID != "3" || call.action != authz.ActionRead {
			t.Fatalf("unexpected status authorization: %+v", call)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUploadStatusMissingAndExpiredRemainUnknown(t *testing.T) {
	for _, scenario := range []string{"missing", "expired", "invalid-state"} {
		t.Run(scenario, func(t *testing.T) {
			authorizer := &uploadStatusAuthorizer{}
			u, mock := newUploadStatusTest(t, authorizer)
			state, expiresAt, receipt := scenario, time.Now().UTC().Add(time.Hour), ""
			if scenario == "expired" {
				state, expiresAt = "committed", time.Now().UTC().Add(-time.Hour)
				receipt = `{"id":900,"name":"converted","type":"file","libraryId":3}`
			}
			expectUploadStatusRow(mock, state, expiresAt, receipt)
			out, err := u.ReconcileCompletion(context.Background(), actor.Actor{ID: "42", Kind: actor.KindUser}, "operation-fixture")
			if err != nil || out.State != UploadCompletionStateUnknown || out.Node != nil {
				t.Fatalf("absence must not establish non-commit: %+v %v", out, err)
			}
			if scenario == "missing" && len(authorizer.calls) != 0 {
				t.Fatal("missing and foreign-actor operations must not reveal library scope")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUploadStatusRevokedLibraryRejectsFoundOperation(t *testing.T) {
	for _, state := range []string{"pending", "committed"} {
		t.Run(state, func(t *testing.T) {
			authorizer := &uploadStatusAuthorizer{err: authz.ErrPermissionDenied}
			u, mock := newUploadStatusTest(t, authorizer)
			receipt := ""
			if state == "committed" {
				receipt = `{"id":900,"name":"converted","type":"file","libraryId":3}`
			}
			expectUploadStatusRow(mock, state, time.Now().UTC().Add(time.Hour), receipt)
			out, err := u.ReconcileCompletion(context.Background(), actor.Actor{ID: "42", Kind: actor.KindUser}, "operation-fixture")
			if !errors.Is(err, authz.ErrPermissionDenied) || out.Node != nil || out.State != "" {
				t.Fatalf("revoked library must not reveal a receipt: %+v %v", out, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
