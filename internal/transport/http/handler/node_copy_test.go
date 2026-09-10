package handler

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCopyNodeBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		body, query string
		status      int
	}{
		{`{"libraryId":3,"parentId":10,"name":"a.txt","recursive":true,"conflictPolicy":"auto_rename"}`, "true", 500},
		{`{"libraryId":3}`, "true", 400},
		{`{"libraryId":3,"parentId":10,"conflictPolicy":"replace"}`, "false", 400},
		{`{"libraryId":3,"parentId":10}`, "invalid", 400},
	} {
		r := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(r)
		ctx.Request = httptest.NewRequest("POST", "/api/v1/nodes/9/copy?dryRun="+test.query, strings.NewReader(test.body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Params = gin.Params{{Key: "nodeId", Value: "9"}}
		NewNodeHandler(nil).CopyNode(ctx)
		if r.Code != test.status {
			t.Fatalf("%s: %d %s", test.body, r.Code, r.Body)
		}
		if test.query == "true" && r.Header().Get(dryRunHeaderKey) != "true" {
			t.Fatal("missing dry run header")
		}
		var body map[string]any
		if err := json.Unmarshal(r.Body.Bytes(), &body); err != nil || body["code"] == nil || body["message"] == nil {
			t.Fatalf("invalid envelope: %s", r.Body)
		}
	}
}
