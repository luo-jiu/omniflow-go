package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetadataQueryRejectsInvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{`{}`, `{"libraryId":3,"limit":101}`, `{"libraryId":3,"mode":"wrong"}`, `{"libraryId":3,"nodeType":"link"}`} {
		engine := gin.New()
		engine.POST("/query", (&NodeHandler{}).BrowseMetadata)
		request := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status %d for %s: %s", response.Code, body, response.Body.String())
		}
	}
}
