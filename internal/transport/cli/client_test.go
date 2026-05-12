package cli

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestCreateNodeIncludesConflictPolicy(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/nodes" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("dryRun"); got != "true" {
			t.Fatalf("expected dryRun=true, got %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		bodyText := string(body)
		if !strings.Contains(bodyText, `"conflictPolicy":"auto_rename"`) {
			t.Fatalf("expected conflictPolicy in request body, got %s", bodyText)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"id":9,"name":"docs (1)","type":"dir","parentId":0,"libraryId":1},"request_id":"req-node-create"}`)),
		}, nil
	})

	created, err := client.CreateNode(context.Background(), CreateNodeRequest{
		Name:           "docs",
		Type:           0,
		LibraryID:      1,
		ConflictPolicy: "auto_rename",
	}, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.Name != "docs (1)" {
		t.Fatalf("expected renamed node name, got %q", created.Name)
	}
}

func TestCaptureResourceMonitorSample(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/resource-monitor/samples" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("libraryId"); got != "7" {
			t.Fatalf("expected libraryId=7, got %q", got)
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
			Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"id":5,"dryRun":true,"actorId":"42","scope":"library","libraryId":7,"generatedAt":"2026-05-11T00:00:00Z","providerCount":1,"bucketCount":1,"objectCount":2,"fileRefCount":2,"physicalBytes":1024,"probeTotal":2,"probeOk":1,"probeError":1,"probeUnknown":0,"createdAt":"2026-05-11T00:00:01Z"},"request_id":"req-resource-sample"}`)),
		}, nil
	})

	got, err := client.CaptureResourceMonitorSample(context.Background(), 7, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.ID != 5 {
		t.Fatalf("expected id=5, got %d", got.ID)
	}
	if !got.DryRun {
		t.Fatalf("expected dryRun=true")
	}
	if got.Scope != "library" || got.LibraryID != 7 {
		t.Fatalf("unexpected scope = %s/%d", got.Scope, got.LibraryID)
	}
	if got.PhysicalBytes != 1024 || got.ProbeOK != 1 || got.ProbeError != 1 {
		t.Fatalf("unexpected sample metrics = %+v", got)
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

func TestImportBrowserBookmarks(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/browser-bookmarks/import" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("dryRun"); got != "true" {
			t.Fatalf("expected dryRun=true, got %q", got)
		}
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		body := string(bodyBytes)
		if !strings.Contains(body, `"source":"chrome-local"`) || !strings.Contains(body, `"title":"Example"`) {
			t.Fatalf("unexpected request body: %s", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"dryRun":true,"result":{"importedCount":12}},"request_id":"req-browser-bookmark-import"}`)),
		}, nil
	})

	result, err := client.ImportBrowserBookmarks(context.Background(), BrowserBookmarkImportRequest{
		Source: "chrome-local",
		Items: []BrowserBookmarkImportItem{
			{Kind: "url", Title: "Example", URL: "https://example.com"},
		},
	}, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ImportedCount != 12 {
		t.Fatalf("expected importedCount=12, got %d", result.ImportedCount)
	}
}

func TestUploadInitSendsPayload(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/upload/init" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("expected auth header, got %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		text := string(body)
		if !strings.Contains(text, `"libraryId":1`) {
			t.Fatalf("expected libraryId in body, got %s", text)
		}
		if !strings.Contains(text, `"fileName":"big.bin"`) {
			t.Fatalf("expected fileName in body, got %s", text)
		}
		if !strings.Contains(text, `"fileSize":1048576`) {
			t.Fatalf("expected fileSize in body, got %s", text)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"code":"0","data":{"uploadId":"u-1","storageKey":"libraries/1/x.bin","mode":"single","partSize":1048576,"totalParts":1,"expiresAt":"2026-05-08T00:00:00Z"}}`,
			)),
		}, nil
	})

	res, err := client.UploadInit(context.Background(), UploadInitRequest{
		LibraryID: 1,
		FileName:  "big.bin",
		FileSize:  1024 * 1024,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.UploadID != "u-1" || res.Mode != "single" || res.TotalParts != 1 {
		t.Fatalf("unexpected init result: %+v", res)
	}
}

func TestUploadSignPartsBody(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/upload/parts/sign" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		text := string(body)
		if !strings.Contains(text, `"uploadId":"u-1"`) {
			t.Fatalf("expected uploadId, got %s", text)
		}
		if !strings.Contains(text, `"partNumbers":[1,2,3]`) {
			t.Fatalf("expected partNumbers, got %s", text)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"code":"0","data":{"parts":[{"partNumber":1,"url":"http://minio/u1","expiresAt":"2026-05-07T01:00:00Z"}],"expiresAt":"2026-05-07T01:00:00Z"}}`,
			)),
		}, nil
	})

	res, err := client.UploadSignParts(context.Background(), UploadSignPartsRequest{
		UploadID:    "u-1",
		PartNumbers: []int{1, 2, 3},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Parts) != 1 || res.Parts[0].URL != "http://minio/u1" {
		t.Fatalf("unexpected sign result: %+v", res)
	}
}

func TestUploadCompleteIncludesConflictPolicy(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/upload/complete" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		text := string(body)
		if !strings.Contains(text, `"conflictPolicy":"auto_rename"`) {
			t.Fatalf("expected conflictPolicy in body, got %s", text)
		}
		if !strings.Contains(text, `"partNumber":1`) || !strings.Contains(text, `"etag":"abc"`) {
			t.Fatalf("expected parts in body, got %s", text)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"code":"0","data":{"id":42,"name":"big.bin","type":"file","parentId":0,"libraryId":1,"fileSize":100}}`,
			)),
		}, nil
	})

	node, err := client.UploadComplete(context.Background(), UploadCompleteRequest{
		UploadID:       "u-1",
		Parts:          []UploadCompletedPart{{PartNumber: 1, ETag: "abc"}},
		ConflictPolicy: "auto_rename",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.ID != 42 {
		t.Fatalf("unexpected node id: %d", node.ID)
	}
}

func TestUploadAbortPathEscaped(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/upload/u-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	})

	if err := client.UploadAbort(context.Background(), "u-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPresignedPutReturnsETag(t *testing.T) {
	t.Parallel()

	client := NewClient("http://example.test", "tester", "token-123")
	// PresignedPut uses its own http.Client so we install a server instead.
	server := newTestPresignedServer(t)
	defer server.close()

	etag, err := client.PresignedPut(
		context.Background(),
		server.url,
		strings.NewReader("hello"),
		5,
		"application/octet-stream",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if etag != "etag-123" {
		t.Fatalf("expected etag-123, got %q", etag)
	}
	if server.method != http.MethodPut {
		t.Fatalf("expected PUT, got %s", server.method)
	}
	if server.contentLength != 5 {
		t.Fatalf("expected Content-Length=5, got %d", server.contentLength)
	}
	if server.contentType != "application/octet-stream" {
		t.Fatalf("expected content type, got %q", server.contentType)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type testPresignedServer struct {
	server        *httptest.Server
	url           string
	method        string
	contentLength int64
	contentType   string
}

func (s *testPresignedServer) close() { s.server.Close() }

func newTestPresignedServer(t *testing.T) *testPresignedServer {
	t.Helper()

	s := &testPresignedServer{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.method = r.Method
		s.contentLength = r.ContentLength
		s.contentType = r.Header.Get("Content-Type")
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("ETag", `"etag-123"`)
		w.WriteHeader(http.StatusOK)
	}))
	s.url = s.server.URL
	return s
}
