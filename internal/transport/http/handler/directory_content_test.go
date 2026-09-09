package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConditionalContentRequiresPrecondition(t *testing.T) {
	for _, body := range []string{`{"libraryId":3,"content":""}`, `{"libraryId":3,"content":"x","expectedStorageKey":null}`} {
		router := gin.New()
		router.PUT("/nodes/:nodeId/content/conditional", NewDirectoryHandler(nil).UpdateFileContentConditional)
		response := httptest.NewRecorder()
		request := httptest.NewRequest("PUT", "/nodes/9/content/conditional?dryRun=true", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(response, request)
		if response.Code != 400 || !strings.Contains(response.Body.String(), "expectedStorageKey") {
			t.Fatalf("unexpected response %d: %s", response.Code, response.Body.String())
		}
	}
}
