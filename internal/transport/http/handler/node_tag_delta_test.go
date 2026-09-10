package handler

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNodeTagDeltaBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		body, query string
		status      int
	}{
		{`{"libraryId":3,"addTagIds":[1],"removeTagIds":[2]}`, "true", 500},
		{`{"addTagIds":[1]}`, "false", 400},
		{`{"libraryId":3,"addTagIds":[0]}`, "false", 400},
		{`{"libraryId":3,"addTagIds":[1]}`, "invalid", 400},
	} {
		r := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(r)
		ctx.Request = httptest.NewRequest("PATCH", "/api/v1/nodes/9/tags?dryRun="+test.query, strings.NewReader(test.body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Params = gin.Params{{Key: "nodeId", Value: "9"}}
		NewNodeHandler(nil).UpdateNodeTags(ctx)
		if r.Code != test.status {
			t.Fatalf("%d %s", r.Code, r.Body)
		}
		if test.query == "true" && r.Header().Get(dryRunHeaderKey) != "true" {
			t.Fatal("missing dry-run header")
		}
		var envelope map[string]any
		if json.Unmarshal(r.Body.Bytes(), &envelope) != nil || envelope["code"] == nil || envelope["message"] == nil {
			t.Fatal(r.Body)
		}
	}
}

func TestNodeTagDeltaReadRequiresLibrary(t *testing.T) {
	r := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(r)
	ctx.Request = httptest.NewRequest("GET", "/api/v1/nodes/9/tags", nil)
	ctx.Params = gin.Params{{Key: "nodeId", Value: "9"}}
	NewNodeHandler(nil).ReadNodeTags(ctx)
	if r.Code != 400 {
		t.Fatal(r.Code)
	}
}
