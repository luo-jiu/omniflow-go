package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCopyNodeClientContract(t *testing.T) {
	client := NewClient("http://example.test", "tester", "test-token")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/nodes/9/copy" || r.URL.Query().Get("dryRun") != "true" {
			t.Fatalf("invalid request: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("username") != "tester" {
			t.Fatal("missing auth")
		}
		var req CopyNodeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.LibraryID != 3 || req.ParentID != 10 || req.Name != "a.txt" || !req.Recursive || req.ConflictPolicy != "auto_rename" {
			t.Fatal(req)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"node":{"id":0},"copiedCount":2,"affectedParentIds":[10],"dryRun":true},"request_id":"copy-test"}`))}, nil
	})
	out, err := client.CopyNode(context.Background(), 9, CopyNodeRequest{LibraryID: 3, ParentID: 10, Name: "a.txt", Recursive: true, ConflictPolicy: "auto_rename"}, true)
	if err != nil || out["copiedCount"] != float64(2) {
		t.Fatalf("%v %v", out, err)
	}
}

func TestCopyCLIHelpAndArguments(t *testing.T) {
	out, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app := NewApp(out, stderr)
	if app.Run([]string{"help", "fs", "cp", "--examples"}) != 0 {
		t.Fatal(stderr.String())
	}
	for _, flag := range []string{"--recursive", "--dry-run", "--conflict-policy", "--parent-id", "--json"} {
		if !strings.Contains(out.String(), flag) {
			t.Fatal(out.String())
		}
	}
	for _, args := range [][]string{{"fs", "cp"}, {"fs", "cp", "extra"}, {"fs", "cp", "--node-id", "9", "--library-id", "3", "--parent-id", "10", "--conflict-policy", "replace"}} {
		if app.Run(args) == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
}
