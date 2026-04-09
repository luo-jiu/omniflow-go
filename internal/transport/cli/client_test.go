package cli

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
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

func TestBatchSetArchiveChildrenBuiltInType(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/nodes/123/archive/built-in-type/batch-set" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("dryRun"); got != "true" {
			t.Fatalf("expected dryRun=true, got %q", got)
		}
		if got := r.Header.Get("username"); got != "tester" {
			t.Fatalf("expected username header to be set, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("expected authorization header to be set, got %q", got)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"nodeId":123,"libraryId":1,"builtInType":"COMIC","totalChildren":5,"dirChildren":3,"updatedCount":2},"request_id":"req-1"}`)),
		}, nil
	})
	result, err := client.BatchSetArchiveChildrenBuiltInType(context.Background(), 123, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.NodeID != 123 {
		t.Fatalf("expected nodeId=123, got %d", result.NodeID)
	}
	if result.LibraryID != 1 {
		t.Fatalf("expected libraryId=1, got %d", result.LibraryID)
	}
	if result.BuiltInType != "COMIC" {
		t.Fatalf("expected builtInType=COMIC, got %s", result.BuiltInType)
	}
	if result.TotalChildren != 5 {
		t.Fatalf("expected totalChildren=5, got %d", result.TotalChildren)
	}
	if result.DirChildren != 3 {
		t.Fatalf("expected dirChildren=3, got %d", result.DirChildren)
	}
	if result.UpdatedCount != 2 {
		t.Fatalf("expected updatedCount=2, got %d", result.UpdatedCount)
	}
}

func TestBatchSetArchiveChildrenBuiltInTypeWithoutDryRun(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("dryRun"); got != "" {
			t.Fatalf("expected dryRun to be omitted, got %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"nodeId":123,"libraryId":1,"builtInType":"COMIC","totalChildren":5,"dirChildren":3,"updatedCount":2},"request_id":"req-1"}`)),
		}, nil
	})

	_, err := client.BatchSetArchiveChildrenBuiltInType(context.Background(), 123, false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
