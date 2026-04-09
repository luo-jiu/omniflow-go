package usecase

import (
	"context"
	"errors"
	"testing"
)

func TestTagUseCaseWithinMutationTx(t *testing.T) {
	t.Parallel()

	t.Run("execute mode runs callback without transactor", func(t *testing.T) {
		t.Parallel()

		u := &TagUseCase{}
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

		u := &TagUseCase{}
		err := u.withinMutationTx(context.Background(), true, func(ctx context.Context) error { return nil })
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("expected ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("dry run uses transaction and swallows rollback marker", func(t *testing.T) {
		t.Parallel()

		tx := &fakeTransactor{}
		u := &TagUseCase{tx: tx}
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
		u := &TagUseCase{tx: tx}
		err := u.withinMutationTx(context.Background(), true, func(ctx context.Context) error {
			return expectedErr
		})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected %v, got %v", expectedErr, err)
		}
	})
}

func TestUserUseCaseWithinMutationTx(t *testing.T) {
	t.Parallel()

	t.Run("execute mode runs callback without transactor", func(t *testing.T) {
		t.Parallel()

		u := &UserUseCase{}
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

		u := &UserUseCase{}
		err := u.withinMutationTx(context.Background(), true, func(ctx context.Context) error { return nil })
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("expected ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("dry run uses transaction and swallows rollback marker", func(t *testing.T) {
		t.Parallel()

		tx := &fakeTransactor{}
		u := &UserUseCase{tx: tx}
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
		u := &UserUseCase{tx: tx}
		err := u.withinMutationTx(context.Background(), true, func(ctx context.Context) error {
			return expectedErr
		})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected %v, got %v", expectedErr, err)
		}
	})
}
