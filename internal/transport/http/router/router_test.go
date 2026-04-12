package router

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"omniflow-go/internal/audit"
	"omniflow-go/internal/authz"
	"omniflow-go/internal/config"
	httpHandler "omniflow-go/internal/transport/http/handler"
	"omniflow-go/internal/usecase"
)

func newTestEngine() http.Handler {
	cfg := &config.Config{
		App: config.App{
			Name:    "omniflow-go",
			Env:     "test",
			Version: "0.1.0",
		},
		Server: config.Server{
			Mode: "release",
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authorizer := authz.NewAllowAll()
	auditSink := audit.NewLogSink(logger)

	nodeUseCase := usecase.NewNodeUseCase(nil, nil, authorizer, auditSink)
	healthHandler := httpHandler.NewHealthHandler(usecase.NewHealthUseCase(cfg))
	authHandler := httpHandler.NewAuthHandler(usecase.NewAuthUseCase(nil, nil, auditSink))
	userHandler := httpHandler.NewUserHandler(nil)
	libraryHandler := httpHandler.NewLibraryHandler(usecase.NewLibraryUseCase(nil, nil, authorizer, auditSink))
	nodeHandler := httpHandler.NewNodeHandler(nodeUseCase)
	directoryHandler := httpHandler.NewDirectoryHandler(usecase.NewDirectoryUseCase(nodeUseCase, nil, authorizer, auditSink))
	fileHandler := httpHandler.NewFileHandler(nil)
	tagHandler := httpHandler.NewTagHandler(nil)
	browserFileMappingHandler := httpHandler.NewBrowserFileMappingHandler(nil)

	engine := New(cfg, logger, healthHandler, authHandler, userHandler, libraryHandler, nodeHandler, directoryHandler, fileHandler, tagHandler, browserFileMappingHandler)
	return engine
}

func TestHealthRoutes(t *testing.T) {
	cfg := &config.Config{
		App: config.App{
			Name:    "omniflow-go",
			Env:     "test",
			Version: "0.1.0",
		},
		Server: config.Server{
			Mode: "release",
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authorizer := authz.NewAllowAll()
	auditSink := audit.NewLogSink(logger)

	nodeUseCase := usecase.NewNodeUseCase(nil, nil, authorizer, auditSink)
	healthHandler := httpHandler.NewHealthHandler(usecase.NewHealthUseCase(cfg))
	authHandler := httpHandler.NewAuthHandler(usecase.NewAuthUseCase(nil, nil, auditSink))
	userHandler := httpHandler.NewUserHandler(nil)
	libraryHandler := httpHandler.NewLibraryHandler(usecase.NewLibraryUseCase(nil, nil, authorizer, auditSink))
	nodeHandler := httpHandler.NewNodeHandler(nodeUseCase)
	directoryHandler := httpHandler.NewDirectoryHandler(usecase.NewDirectoryUseCase(nodeUseCase, nil, authorizer, auditSink))
	fileHandler := httpHandler.NewFileHandler(nil)
	tagHandler := httpHandler.NewTagHandler(nil)
	browserFileMappingHandler := httpHandler.NewBrowserFileMappingHandler(nil)

	engine := New(cfg, logger, healthHandler, authHandler, userHandler, libraryHandler, nodeHandler, directoryHandler, fileHandler, tagHandler, browserFileMappingHandler)

	testCases := []string{"/healthz", "/api/v1/health"}
	for _, path := range testCases {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()

		engine.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected %s to return 200, got %d", path, recorder.Code)
		}
	}
}

func TestProtectedRouteRequiresAuthHeaders(t *testing.T) {
	cfg := &config.Config{
		App: config.App{
			Name:    "omniflow-go",
			Env:     "test",
			Version: "0.1.0",
		},
		Server: config.Server{
			Mode: "release",
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authorizer := authz.NewAllowAll()
	auditSink := audit.NewLogSink(logger)

	nodeUseCase := usecase.NewNodeUseCase(nil, nil, authorizer, auditSink)
	healthHandler := httpHandler.NewHealthHandler(usecase.NewHealthUseCase(cfg))
	authHandler := httpHandler.NewAuthHandler(usecase.NewAuthUseCase(nil, nil, auditSink))
	userHandler := httpHandler.NewUserHandler(nil)
	libraryHandler := httpHandler.NewLibraryHandler(usecase.NewLibraryUseCase(nil, nil, authorizer, auditSink))
	nodeHandler := httpHandler.NewNodeHandler(nodeUseCase)
	directoryHandler := httpHandler.NewDirectoryHandler(usecase.NewDirectoryUseCase(nodeUseCase, nil, authorizer, auditSink))
	fileHandler := httpHandler.NewFileHandler(nil)
	tagHandler := httpHandler.NewTagHandler(nil)
	browserFileMappingHandler := httpHandler.NewBrowserFileMappingHandler(nil)

	engine := New(cfg, logger, healthHandler, authHandler, userHandler, libraryHandler, nodeHandler, directoryHandler, fileHandler, tagHandler, browserFileMappingHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected-probe", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected protected route to return 401 without auth, got %d", recorder.Code)
	}

	authedReq := httptest.NewRequest(http.MethodGet, "/api/v1/protected-probe", nil)
	authedReq.Header.Set("Authorization", "Bearer test-token")
	authedReq.Header.Set("username", "1")
	authedRecorder := httptest.NewRecorder()
	engine.ServeHTTP(authedRecorder, authedReq)
	if authedRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected protected route to return 404 with auth, got %d", authedRecorder.Code)
	}
}

func TestAuthStatusRouteRequiresAuthHeaders(t *testing.T) {
	cfg := &config.Config{
		App: config.App{
			Name:    "omniflow-go",
			Env:     "test",
			Version: "0.1.0",
		},
		Server: config.Server{
			Mode: "release",
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authorizer := authz.NewAllowAll()
	auditSink := audit.NewLogSink(logger)

	nodeUseCase := usecase.NewNodeUseCase(nil, nil, authorizer, auditSink)
	healthHandler := httpHandler.NewHealthHandler(usecase.NewHealthUseCase(cfg))
	authHandler := httpHandler.NewAuthHandler(usecase.NewAuthUseCase(nil, nil, auditSink))
	userHandler := httpHandler.NewUserHandler(nil)
	libraryHandler := httpHandler.NewLibraryHandler(usecase.NewLibraryUseCase(nil, nil, authorizer, auditSink))
	nodeHandler := httpHandler.NewNodeHandler(nodeUseCase)
	directoryHandler := httpHandler.NewDirectoryHandler(usecase.NewDirectoryUseCase(nodeUseCase, nil, authorizer, auditSink))
	fileHandler := httpHandler.NewFileHandler(nil)
	tagHandler := httpHandler.NewTagHandler(nil)
	browserFileMappingHandler := httpHandler.NewBrowserFileMappingHandler(nil)

	engine := New(cfg, logger, healthHandler, authHandler, userHandler, libraryHandler, nodeHandler, directoryHandler, fileHandler, tagHandler, browserFileMappingHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status?username=test&token=test", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected /api/v1/auth/status to return 401 without auth headers, got %d", recorder.Code)
	}
}

func TestBrowserFileMappingResolveRouteIsRegistered(t *testing.T) {
	engine := newTestEngine()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/browser-file-mappings/resolve?fileExt=txt", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("username", "1")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected browser mapping resolve route to hit handler and return 500 when service is nil, got %d", recorder.Code)
	}
}

func TestBrowserFileMappingDeleteDryRunMarksHeader(t *testing.T) {
	engine := newTestEngine()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/browser-file-mappings/7?dryRun=true", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("username", "1")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected browser mapping delete route to hit handler and return 500 when service is nil, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("X-Omniflow-Dry-Run"); got != "true" {
		t.Fatalf("expected dry-run header=true, got %q", got)
	}
}
