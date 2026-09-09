package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestWriteFileContentClient(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		for _, condition := range []*string{nil, new(string)} {
			client := NewClient("http://example.test", "tester", "test-token")
			client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				path := "/api/v1/nodes/9/content"
				if condition != nil {
					path += "/conditional"
				}
				if r.Method != "PUT" || r.URL.Path != path || (r.URL.Query().Get("dryRun") == "true") != dryRun {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Fatal("missing authentication")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				value, present := body["expectedStorageKey"]
				if present != (condition != nil) || (present && value != "") || body["libraryId"] != float64(3) || body["content"] != "hello" {
					t.Fatalf("unexpected body: %#v", body)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"code":"0","data":{"id":9}}`))}, nil
			})
			node, err := client.WriteFileContent(context.Background(), 9, WriteFileContentRequest{LibraryID: 3, Content: "hello", ExpectedStorageKey: condition}, dryRun)
			if err != nil || node.ID != 9 {
				t.Fatalf("node=%v err=%v", node, err)
			}
		}
	}
}
