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

func TestNodeTagDeltaClientContract(t *testing.T) {
	client := NewClient("http://example.test", "tester", "test-token")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/nodes/9/tags" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal(r.URL)
		}
		if r.Method == "PATCH" {
			if r.URL.Query().Get("dryRun") != "true" {
				t.Fatal(r.URL)
			}
			var input NodeTagDeltaRequest
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if input.LibraryID != 3 || len(input.AddTagIDs) != 1 || input.AddTagIDs[0] != 2 {
				t.Fatal(input)
			}
		} else if r.Method != "GET" || r.URL.Query().Get("libraryId") != "3" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"code":"0","data":{"nodeId":9,"tagIds":[2]}}`))}, nil
	})
	if _, err := client.UpdateNodeTags(context.Background(), 9, NodeTagDeltaRequest{LibraryID: 3, AddTagIDs: []uint64{2}}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReadNodeTags(context.Background(), 9, 3); err != nil {
		t.Fatal(err)
	}
}

func TestNodeTagDeltaCLIArguments(t *testing.T) {
	out, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app := NewApp(out, stderr)
	if app.Run([]string{"help", "fs", "tags-update", "--examples"}) != 0 {
		t.Fatal(stderr.String())
	}
	for _, flag := range []string{"--dry-run", "--add-tag-ids", "--remove-tag-ids", "--json"} {
		if !strings.Contains(out.String(), flag) {
			t.Fatal(out.String())
		}
	}
	for _, args := range [][]string{{"fs", "tags"}, {"fs", "tags", "extra"}, {"fs", "tags-update", "--library-id", "3", "--node-id", "9"},
		{"fs", "tags-update", "--library-id", "3", "--node-id", "9", "--add-tag-ids", "1", "--remove-tag-ids", "1"}} {
		if app.Run(args) == 0 {
			t.Fatal(args)
		}
	}
}
