package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	domainnode "omniflow-go/internal/domain/node"
	"strings"
	"testing"
)

func TestBrowseMetadataClientContract(t *testing.T) {
	client := NewClient("http://example.test", "tester", "token-fixture")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/nodes/metadata/query" || r.Header.Get("Authorization") != "Bearer token-fixture" {
			t.Fatalf("unexpected request: %v", r)
		}
		var body MetadataQueryRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.LibraryID != 3 || body.Mode != "search" || body.Cursor != "cursor" || body.Limit != 20 || body.Keyword != "music" {
			t.Fatalf("bad body: %+v", body)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"root":{"id":10,"libraryId":3,"parentId":0,"name":"root","type":"dir","path":"/","fileSize":0},"entries":[],"hasMore":true,"nextCursor":"next"},"request_id":"fixture"}`))}, nil
	})
	page, err := client.BrowseMetadata(context.Background(), MetadataQueryRequest{MetadataQuery: domainnode.MetadataQuery{LibraryID: 3, Mode: "search", Keyword: "music"}, Cursor: "cursor", Limit: 20})
	if err != nil || !page.HasMore || page.NextCursor != "next" {
		t.Fatalf("bad page: %+v %v", page, err)
	}
}

func TestBrowseMetadataCLIValidation(t *testing.T) {
	for _, args := range [][]string{{"--library-id", "3", "extra"}, {"--library-id", "3", "--limit", "101"}, {"--library-id", "3", "--mode", "wrong"}} {
		out, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		if NewApp(out, stderr).Run(append([]string{"fs", "browse"}, args...)) == 0 {
			t.Fatalf("accepted invalid args: %v", args)
		}
	}
}
