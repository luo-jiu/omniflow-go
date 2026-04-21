package repository

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapDBErrorOnlyMapsNodeNameUniqueConstraintToConflict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "node sibling name unique constraint maps to conflict",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "uq_nodes_live_sibling_visible_name",
			},
			want: ErrConflict,
		},
		{
			name: "other unique constraint keeps original error",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "uq_storage_objects_live_locator",
			},
			want: nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := mapDBError(test.err)
			if test.want != nil {
				if !errors.Is(got, test.want) {
					t.Fatalf("expected %v, got %v", test.want, got)
				}
				return
			}
			if !errors.Is(got, test.err) {
				t.Fatalf("expected original error to be preserved, got %v", got)
			}
		})
	}
}
