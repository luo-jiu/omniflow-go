package usecase

import (
	"context"
	"errors"
	"testing"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
)

func TestBrowserFileMappingUseCaseWithinMutationTx(t *testing.T) {
	t.Parallel()

	t.Run("execute mode runs callback without transactor", func(t *testing.T) {
		t.Parallel()

		u := &BrowserFileMappingUseCase{}
		called := 0
		err := u.withinMutationTx(context.Background(), false, func(ctx context.Context) error {
			called++
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if called != 1 {
			t.Fatalf("expected callback to be called once, got %d", called)
		}
	})

	t.Run("dry run without transactor returns invalid argument", func(t *testing.T) {
		t.Parallel()

		u := &BrowserFileMappingUseCase{}
		err := u.withinMutationTx(context.Background(), true, func(ctx context.Context) error { return nil })
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("expected ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("dry run uses transaction and swallows rollback marker", func(t *testing.T) {
		t.Parallel()

		tx := &fakeTransactor{}
		u := &BrowserFileMappingUseCase{tx: tx}
		called := 0
		err := u.withinMutationTx(context.Background(), true, func(ctx context.Context) error {
			called++
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if called != 1 {
			t.Fatalf("expected callback to be called once, got %d", called)
		}
		if tx.calls != 1 {
			t.Fatalf("expected transactor calls=1, got %d", tx.calls)
		}
	})

	t.Run("dry run propagates business error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("boom")
		tx := &fakeTransactor{}
		u := &BrowserFileMappingUseCase{tx: tx}
		err := u.withinMutationTx(context.Background(), true, func(ctx context.Context) error {
			return expectedErr
		})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected %v, got %v", expectedErr, err)
		}
	})
}

func TestNormalizeBrowserFileExt(t *testing.T) {
	t.Parallel()

	got, err := normalizeBrowserFileExt(".Excalidraw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "excalidraw" {
		t.Fatalf("expected ext=excalidraw, got %s", got)
	}

	if _, err := normalizeBrowserFileExt("bad ext"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument for invalid ext, got %v", err)
	}
}

func TestNormalizeBrowserSiteURL(t *testing.T) {
	t.Parallel()

	got, err := normalizeBrowserSiteURL(" https://excalidraw.com ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://excalidraw.com" {
		t.Fatalf("expected normalized url, got %s", got)
	}

	if _, err := normalizeBrowserSiteURL("ftp://example.test"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument for unsupported scheme, got %v", err)
	}
}

type captureAuditSink struct {
	events []audit.Event
}

func (s *captureAuditSink) Write(_ context.Context, event audit.Event) error {
	s.events = append(s.events, event)
	return nil
}

func TestBrowserFileMappingUseCaseWriteAudit(t *testing.T) {
	t.Parallel()

	t.Run("skips when audit sink is nil", func(t *testing.T) {
		t.Parallel()

		u := &BrowserFileMappingUseCase{}
		err := u.writeAudit(context.Background(), actor.Actor{ID: "1", Kind: actor.KindUser}, "browser_file_mapping.create", true, map[string]any{
			"mode": "execute",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("writes browser file mapping audit event", func(t *testing.T) {
		t.Parallel()

		sink := &captureAuditSink{}
		u := &BrowserFileMappingUseCase{auditLog: sink}
		principal := actor.Actor{ID: "7", Kind: actor.KindUser}
		metadata := map[string]any{
			"mapping_id":    42,
			"owner_user_id": uint64(7),
			"file_ext":      "excalidraw",
			"site_url":      "https://excalidraw.com",
			"mode":          "dry_run",
			"dry_run":       true,
		}

		if err := u.writeAudit(context.Background(), principal, "browser_file_mapping.update", true, metadata); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(sink.events) != 1 {
			t.Fatalf("expected 1 audit event, got %d", len(sink.events))
		}
		event := sink.events[0]
		if event.Actor.ID != principal.ID || event.Actor.Kind != principal.Kind {
			t.Fatalf("unexpected actor: %+v", event.Actor)
		}
		if event.Action != "browser_file_mapping.update" {
			t.Fatalf("unexpected action: %s", event.Action)
		}
		if event.Resource != "browser_file_mapping" {
			t.Fatalf("unexpected resource: %s", event.Resource)
		}
		if !event.Success {
			t.Fatalf("expected success=true")
		}
		if event.Metadata["mapping_id"] != 42 {
			t.Fatalf("unexpected mapping_id: %#v", event.Metadata["mapping_id"])
		}
		if event.Metadata["mode"] != "dry_run" {
			t.Fatalf("unexpected mode: %#v", event.Metadata["mode"])
		}
	})
}
