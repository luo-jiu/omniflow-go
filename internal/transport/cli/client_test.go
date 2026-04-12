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

func TestClearRecycleBin(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/nodes/recycle/library/7/clear" {
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
			Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"clearedCount":3},"request_id":"req-2"}`)),
		}, nil
	})

	clearedCount, err := client.ClearRecycleBin(context.Background(), 7, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if clearedCount != 3 {
		t.Fatalf("expected clearedCount=3, got %d", clearedCount)
	}
}

func TestClearRecycleBinWithoutDryRun(t *testing.T) {
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
			Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"clearedCount":0},"request_id":"req-3"}`)),
		}, nil
	})

	clearedCount, err := client.ClearRecycleBin(context.Background(), 7, false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if clearedCount != 0 {
		t.Fatalf("expected clearedCount=0, got %d", clearedCount)
	}
}

func TestResolveBrowserFileMapping(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/browser-file-mappings/resolve" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("fileExt"); got != "txt" {
			t.Fatalf("expected fileExt=txt, got %q", got)
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
			Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"id":9,"fileExt":"txt","siteUrl":"https://example.test","ownerUserId":1,"createdAt":"2026-04-12T00:00:00Z","updatedAt":"2026-04-12T00:00:00Z"},"request_id":"req-browser-resolve"}`)),
		}, nil
	})

	item, err := client.ResolveBrowserFileMapping(context.Background(), "txt")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if item.ID != 9 {
		t.Fatalf("expected id=9, got %d", item.ID)
	}
	if item.FileExt != "txt" {
		t.Fatalf("expected fileExt=txt, got %s", item.FileExt)
	}
	if item.SiteURL != "https://example.test" {
		t.Fatalf("expected siteUrl to match, got %s", item.SiteURL)
	}
}

func TestCreateBrowserFileMapping(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/browser-file-mappings" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("dryRun"); got != "true" {
			t.Fatalf("expected dryRun=true, got %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"id":11,"fileExt":"excalidraw","siteUrl":"https://excalidraw.com","ownerUserId":1,"createdAt":"2026-04-12T00:00:00Z","updatedAt":"2026-04-12T00:00:00Z"},"request_id":"req-browser-create"}`)),
		}, nil
	})

	item, err := client.CreateBrowserFileMapping(context.Background(), BrowserFileMappingUpsertRequest{
		FileExt: "excalidraw",
		SiteURL: "https://excalidraw.com",
	}, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if item.ID != 11 {
		t.Fatalf("expected id=11, got %d", item.ID)
	}
	if item.FileExt != "excalidraw" {
		t.Fatalf("expected fileExt=excalidraw, got %s", item.FileExt)
	}
}

func TestMatchBrowserBookmark(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/browser-bookmarks/match" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("url"); got != "https://example.com/path?utm=1" {
			t.Fatalf("expected url query to match, got %q", got)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"matched":true,"bookmark":{"id":5,"ownerUserId":1,"kind":"url","title":"Example","url":"https://example.com/path","urlMatchKey":"https://example.com/path","sortOrder":1000,"createdAt":"2026-04-12T00:00:00Z","updatedAt":"2026-04-12T00:00:00Z"}},"request_id":"req-browser-bookmark-match"}`)),
		}, nil
	})

	result, err := client.MatchBrowserBookmark(context.Background(), "https://example.com/path?utm=1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Matched || result.Bookmark == nil || result.Bookmark.ID != 5 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestMoveBrowserBookmark(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/browser-bookmarks/9/move" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("dryRun"); got != "true" {
			t.Fatalf("expected dryRun=true, got %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"dryRun":true,"result":{"id":9,"ownerUserId":1,"parentId":2,"kind":"url","title":"Example","url":"https://example.com/path","urlMatchKey":"https://example.com/path","sortOrder":2000,"createdAt":"2026-04-12T00:00:00Z","updatedAt":"2026-04-12T00:00:00Z"}},"request_id":"req-browser-bookmark-move"}`)),
		}, nil
	})

	parentID := uint64(2)
	item, err := client.MoveBrowserBookmark(context.Background(), 9, BrowserBookmarkMoveRequest{
		ParentID: &parentID,
	}, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if item.ID != 9 || item.ParentID == nil || *item.ParentID != 2 {
		t.Fatalf("unexpected bookmark move result: %+v", item)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
