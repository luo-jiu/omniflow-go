package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSuccessWithDryRun(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	SuccessWithDryRun(ctx, true, map[string]any{"name": "demo"})

	if got := rec.Header().Get(dryRunHeaderKey); got != "true" {
		t.Fatalf("expected dry-run header=true, got %q", got)
	}

	var resp Result
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	dataMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", resp.Data)
	}
	if dry, ok := dataMap["dryRun"].(bool); !ok || !dry {
		t.Fatalf("expected dryRun=true in response data, got %v", dataMap["dryRun"])
	}
}

func TestSuccessNoDataWithDryRun(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	SuccessNoDataWithDryRun(ctx, true)

	if got := rec.Header().Get(dryRunHeaderKey); got != "true" {
		t.Fatalf("expected dry-run header=true, got %q", got)
	}

	var resp Result
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	dataMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", resp.Data)
	}
	if dry, ok := dataMap["dryRun"].(bool); !ok || !dry {
		t.Fatalf("expected dryRun=true in response data, got %v", dataMap["dryRun"])
	}
}
