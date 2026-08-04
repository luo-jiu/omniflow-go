package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUpdateNodeDryRunMarksResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/nodes/123?dry_run=true",
		strings.NewReader(`{"builtInType":"COMIC"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "nodeId", Value: "123"}}

	NewNodeHandler(nil).UpdateNode(ctx)

	if got := recorder.Header().Get(dryRunHeaderKey); got != "true" {
		t.Fatalf("expected dry-run response header, got %q", got)
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected nil service to return 500 after binding, got %d", recorder.Code)
	}
}

func TestUpdateNodeRejectsInvalidDryRun(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/nodes/123?dryRun=invalid",
		strings.NewReader(`{"builtInType":"COMIC"}`),
	)
	ctx.Params = gin.Params{{Key: "nodeId", Value: "123"}}

	NewNodeHandler(nil).UpdateNode(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid dryRun to return 400, got %d", recorder.Code)
	}
	if got := recorder.Header().Get(dryRunHeaderKey); got != "" {
		t.Fatalf("expected no dry-run response header, got %q", got)
	}
}
