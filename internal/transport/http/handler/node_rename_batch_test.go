package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"omniflow-go/internal/actor"
	"omniflow-go/internal/transport/http/middleware"
	"omniflow-go/internal/usecase"
)

const renameRequestJSON = `{"libraryId":3,"operationId":"12345678-1234-4234-8234-123456789abc","items":[{"nodeId":9,"name":"new","expected":{"name":"old","ext":"","parentId":8,"updatedAt":"2026-09-22T01:02:03.123456Z"}}]}`

func TestRenameBatchHTTPBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, body, query string
		status            int
	}{
		{"valid", renameRequestJSON, "true", 500},
		{"missing-ext", strings.Replace(renameRequestJSON, `"ext":"",`, "", 1), "true", 400},
		{"null-ext", strings.Replace(renameRequestJSON, `"ext":""`, `"ext":null`, 1), "false", 400},
		{"extra-field", strings.Replace(renameRequestJSON, `"name":"new"`, `"name":"new","ext":"exe"`, 1), "false", 400},
		{"missing-items", `{"libraryId":3,"operationId":"12345678-1234-4234-8234-123456789abc"}`, "false", 400},
		{"bad-time", strings.Replace(renameRequestJSON, "2026-09-22T01:02:03.123456Z", "yesterday", 1), "false", 400},
		{"trailing-json", renameRequestJSON + `{}`, "false", 400},
		{"bad-dry-run", renameRequestJSON, "invalid", 400},
		{"bounded-body", strings.Repeat(" ", 128<<10) + renameRequestJSON, "false", 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			ctx.Request = httptest.NewRequest("POST", "/api/v1/nodes/rename/batch/conditional?dryRun="+test.query, strings.NewReader(test.body))
			NewNodeHandler(nil).RenameNodesConditional(ctx)
			if response.Code != test.status {
				t.Fatalf("%d %s", response.Code, response.Body)
			}
			if test.query == "true" && response.Header().Get(dryRunHeaderKey) != "true" {
				t.Fatal("missing dry-run header")
			}
			var result map[string]any
			if json.Unmarshal(response.Body.Bytes(), &result) != nil || result["code"] == nil || result["request_id"] == nil {
				t.Fatal(response.Body)
			}
		})
	}
}

func TestRenameBatchHTTPValidationAndIdentity(t *testing.T) {
	for _, test := range []struct {
		body      string
		withActor bool
		status    int
	}{
		{renameRequestJSON, false, 401},
		{strings.Replace(renameRequestJSON, `"name":"new"`, `"name":"../escape"`, 1), true, 400},
		{strings.Replace(renameRequestJSON, `"parentId":8`, `"parentId":0`, 1), true, 400},
		{strings.Replace(renameRequestJSON, `12345678-1234-4234-8234-123456789abc`, `not-a-uuid`, 1), true, 400},
	} {
		response := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(response)
		ctx.Request = httptest.NewRequest("POST", "/api/v1/nodes/rename/batch/conditional", strings.NewReader(test.body))
		if test.withActor {
			ctx.Set(middleware.ActorKey, actor.Actor{Kind: actor.KindUser, ID: "7"})
		}
		NewNodeHandler(usecase.NewNodeUseCase(nil, nil, nil, nil)).RenameNodesConditional(ctx)
		if response.Code != test.status {
			t.Fatalf("%d %s", response.Code, response.Body)
		}
	}
}

func TestRenameBatchStatusHTTPRequiresScope(t *testing.T) {
	for _, query := range []string{"", "?libraryId=3", "?operationId=x"} {
		response := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(response)
		ctx.Request = httptest.NewRequest("GET", "/api/v1/nodes/rename/batch/status"+query, nil)
		NewNodeHandler(nil).RenameNodesStatus(ctx)
		if response.Code != 400 {
			t.Fatal(response.Code)
		}
	}
}
