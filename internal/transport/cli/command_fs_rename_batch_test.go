package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domainnode "omniflow-go/internal/domain/node"
)

const renameCLIOperation = "12345678-1234-4234-8234-123456789abc"

func TestRenameBatchClientContract(t *testing.T) {
	client := NewClient("http://example.test", "tester", "test-token")
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("missing authentication")
		}
		if req.Method == "POST" {
			if req.URL.Path != "/api/v1/nodes/rename/batch/conditional" || req.URL.Query().Get("dryRun") != "true" {
				t.Fatal(req.URL)
			}
			var body RenameBatchRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.LibraryID != 3 || body.OperationID != renameCLIOperation || len(body.Items) != 1 ||
				body.Items[0].Expected.Ext != "jpg" || body.Items[0].Expected.ParentID != 8 || body.Items[0].Name != "new" {
				t.Fatal(body)
			}
		} else if req.Method != "GET" || req.URL.Path != "/api/v1/nodes/rename/batch/status" ||
			req.URL.Query().Get("libraryId") != "3" || req.URL.Query().Get("operationId") != renameCLIOperation {
			t.Fatal(req.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"code":"0","data":{"state":"not_found"}}`))}, nil
	})
	_, err := client.RenameBatch(context.Background(), RenameBatchRequest{LibraryID: 3, OperationID: renameCLIOperation,
		Items: []domainnode.RenameBatchItem{{NodeID: 9, Name: "new", Expected: domainnode.RenameExpected{
			Name: "old", Ext: "jpg", ParentID: 8, UpdatedAt: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
		}}}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.RenameBatchStatus(context.Background(), 3, renameCLIOperation); err != nil {
		t.Fatal(err)
	}
}

func TestRenameBatchCLIForwardsItemsAndProjectsValidatedResult(t *testing.T) {
	const items = `[{"nodeId":9,"name":"new","expected":{"name":"old","parentId":8,"updatedAt":"2026-09-22T00:00:00Z"}}]`
	file := filepath.Join(t.TempDir(), "proposal.json")
	if err := os.WriteFile(file, []byte(items), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/nodes/rename/batch/conditional" || r.URL.Query().Get("dryRun") != "true" {
			t.Error(r.URL)
		}
		var body struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Items) != 1 {
			t.Error(err)
		}
		if _, present := body.Items[0]["expected"].(map[string]any)["ext"]; present {
			t.Error("CLI silently filled missing expected ext")
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":"0","message":"success","data":{"dryRun":true,"result":{"state":"validated"}},"request_id":"test"}`)
	}))
	defer server.Close()
	t.Setenv(envToken, "test-token")
	t.Setenv(envUsername, "tester")
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app := NewApp(stdout, stderr)
	code := app.Run([]string{"fs", "rename-batch", "--library-id", "3", "--operation-id", renameCLIOperation,
		"--file", file, "--dry-run", "--json", "--base-url", server.URL})
	if code != 0 || !strings.Contains(stdout.String(), `"state": "validated"`) {
		t.Fatalf("%d %s %s", code, stdout, stderr)
	}
}

func TestRenameBatchCLIHelpAndArgumentFailures(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app := NewApp(stdout, stderr)
	if app.Run([]string{"help", "fs", "rename-batch", "--examples"}) != 0 {
		t.Fatal(stderr)
	}
	for _, flag := range []string{"--operation-id", "--file", "--dry-run", "--json"} {
		if !strings.Contains(stdout.String(), flag) {
			t.Fatal(stdout)
		}
	}
	for _, args := range [][]string{
		{"fs", "rename-batch"}, {"fs", "rename-status"}, {"fs", "rename-batch", "extra"},
		{"fs", "rename-status", "--library-id", "3", "--operation-id", renameCLIOperation, "extra"},
		{"fs", "rename-batch", "--library-id", "3", "--operation-id", renameCLIOperation},
	} {
		if app.Run(args) == 0 {
			t.Fatal(args)
		}
	}
}
