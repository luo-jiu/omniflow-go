package cli

import (
	"net/url"
	"testing"
)

func TestWithDryRunQuery(t *testing.T) {
	t.Parallel()

	t.Run("dry run disabled keeps query unchanged", func(t *testing.T) {
		t.Parallel()

		query := url.Values{}
		query.Set("libraryId", "1")

		got := withDryRunQuery(query, false)
		if got.Get("dryRun") != "" {
			t.Fatalf("expected dryRun to be empty, got %q", got.Get("dryRun"))
		}
		if got.Get("libraryId") != "1" {
			t.Fatalf("expected libraryId=1, got %q", got.Get("libraryId"))
		}
	})

	t.Run("dry run enabled appends query flag", func(t *testing.T) {
		t.Parallel()

		got := withDryRunQuery(nil, true)
		if got.Get("dryRun") != "true" {
			t.Fatalf("expected dryRun=true, got %q", got.Get("dryRun"))
		}
	})
}
