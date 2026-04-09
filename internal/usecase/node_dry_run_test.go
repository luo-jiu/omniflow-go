package usecase

import (
	"context"
	"errors"
	"testing"
)

type fakeTransactor struct {
	calls int
}

func (t *fakeTransactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	t.calls++
	if fn == nil {
		return nil
	}
	return fn(ctx)
}

func TestNodeUseCaseWithinMutationTx(t *testing.T) {
	t.Parallel()

	t.Run("execute mode runs callback directly without transactor", func(t *testing.T) {
		t.Parallel()

		u := &NodeUseCase{}
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

		u := &NodeUseCase{}
		err := u.withinMutationTx(context.Background(), true, func(ctx context.Context) error { return nil })
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("expected ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("dry run uses transaction and swallows rollback marker", func(t *testing.T) {
		t.Parallel()

		tx := &fakeTransactor{}
		u := &NodeUseCase{tx: tx}
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
		u := &NodeUseCase{tx: tx}
		err := u.withinMutationTx(context.Background(), true, func(ctx context.Context) error {
			return expectedErr
		})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected %v, got %v", expectedErr, err)
		}
	})
}
