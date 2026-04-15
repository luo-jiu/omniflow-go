package router

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omniflow-go/internal/audit"
	"omniflow-go/internal/authz"
	"omniflow-go/internal/config"
	domainbrowserbookmark "omniflow-go/internal/domain/browserbookmark"
	"omniflow-go/internal/repository"
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
	browserBookmarkHandler := httpHandler.NewBrowserBookmarkHandler(nil)
	browserFileMappingHandler := httpHandler.NewBrowserFileMappingHandler(nil)

	engine := New(cfg, logger, healthHandler, authHandler, userHandler, libraryHandler, nodeHandler, directoryHandler, fileHandler, tagHandler, browserBookmarkHandler, browserFileMappingHandler)
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
	browserBookmarkHandler := httpHandler.NewBrowserBookmarkHandler(nil)
	browserFileMappingHandler := httpHandler.NewBrowserFileMappingHandler(nil)

	engine := New(cfg, logger, healthHandler, authHandler, userHandler, libraryHandler, nodeHandler, directoryHandler, fileHandler, tagHandler, browserBookmarkHandler, browserFileMappingHandler)

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
	browserBookmarkHandler := httpHandler.NewBrowserBookmarkHandler(nil)
	browserFileMappingHandler := httpHandler.NewBrowserFileMappingHandler(nil)

	engine := New(cfg, logger, healthHandler, authHandler, userHandler, libraryHandler, nodeHandler, directoryHandler, fileHandler, tagHandler, browserBookmarkHandler, browserFileMappingHandler)

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
	browserBookmarkHandler := httpHandler.NewBrowserBookmarkHandler(nil)
	browserFileMappingHandler := httpHandler.NewBrowserFileMappingHandler(nil)

	engine := New(cfg, logger, healthHandler, authHandler, userHandler, libraryHandler, nodeHandler, directoryHandler, fileHandler, tagHandler, browserBookmarkHandler, browserFileMappingHandler)

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

func TestBrowserBookmarkMatchRouteIsRegistered(t *testing.T) {
	engine := newTestEngine()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/browser-bookmarks/match?url=https%3A%2F%2Fexample.com", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("username", "1")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected bookmark match route to hit handler and return 500 when service is nil, got %d", recorder.Code)
	}
}

func TestBrowserBookmarkDeleteDryRunMarksHeader(t *testing.T) {
	engine := newTestEngine()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/browser-bookmarks/7?dryRun=true", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("username", "1")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected bookmark delete route to hit handler and return 500 when service is nil, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("X-Omniflow-Dry-Run"); got != "true" {
		t.Fatalf("expected dry-run header=true, got %q", got)
	}
}

func TestBrowserBookmarkMatchRouteReturnsUnmatchedPayload(t *testing.T) {
	engine := newBookmarkBehaviorTestEngine(&routerTestBrowserBookmarkRepository{
		findFirstURLByMatchKeyFunc: func(ownerUserID uint64, urlMatchKey string) (domainbrowserbookmark.BrowserBookmark, error) {
			return domainbrowserbookmark.BrowserBookmark{}, repository.ErrNotFound
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/browser-bookmarks/match?url=https%3A%2F%2Fexample.com%2Fdemo%2F%3Futm%3D1", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("username", "1")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"matched":false`) {
		t.Fatalf("expected unmatched payload, got %s", body)
	}
}

func TestBrowserBookmarkCreateRouteMapsConflict(t *testing.T) {
	engine := newBookmarkBehaviorTestEngine(&routerTestBrowserBookmarkRepository{
		nextSortOrderFunc: func(ownerUserID uint64, parentID *uint64) (int, error) {
			return 1000, nil
		},
		createFunc: func(input repository.CreateBrowserBookmarkInput) (domainbrowserbookmark.BrowserBookmark, error) {
			return domainbrowserbookmark.BrowserBookmark{}, repository.ErrConflict
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/browser-bookmarks", strings.NewReader(`{"kind":"url","title":"Example","url":"https://example.com/demo?utm=1"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("username", "1")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestBrowserBookmarkDeleteRouteMapsNotFound(t *testing.T) {
	engine := newBookmarkBehaviorTestEngine(&routerTestBrowserBookmarkRepository{
		softDeleteTreeOwnerByIDFunc: func(id, ownerUserID uint64) (bool, error) {
			return false, nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/browser-bookmarks/7", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("username", "1")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestBrowserBookmarkImportRouteMapsInvalidArgument(t *testing.T) {
	engine := newBookmarkBehaviorTestEngine(&routerTestBrowserBookmarkRepository{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/browser-bookmarks/import", strings.NewReader(`{"source":"chrome-local","items":[{"kind":"folder","title":"Bad","url":"https://example.com"}]}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("username", "1")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
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

type routerTestBrowserBookmarkRepository struct {
	findFirstURLByMatchKeyFunc  func(ownerUserID uint64, urlMatchKey string) (domainbrowserbookmark.BrowserBookmark, error)
	nextSortOrderFunc           func(ownerUserID uint64, parentID *uint64) (int, error)
	createFunc                  func(input repository.CreateBrowserBookmarkInput) (domainbrowserbookmark.BrowserBookmark, error)
	softDeleteTreeOwnerByIDFunc func(id, ownerUserID uint64) (bool, error)
}

func (r *routerTestBrowserBookmarkRepository) ListByOwner(ctx context.Context, ownerUserID uint64) ([]domainbrowserbookmark.BrowserBookmark, error) {
	return nil, nil
}

func (r *routerTestBrowserBookmarkRepository) ListSiblings(ctx context.Context, ownerUserID uint64, parentID *uint64, excludeID uint64) ([]domainbrowserbookmark.BrowserBookmark, error) {
	return nil, nil
}

func (r *routerTestBrowserBookmarkRepository) FindOwnerByID(ctx context.Context, id, ownerUserID uint64) (domainbrowserbookmark.BrowserBookmark, error) {
	return domainbrowserbookmark.BrowserBookmark{}, repository.ErrNotFound
}

func (r *routerTestBrowserBookmarkRepository) FindFirstURLByMatchKey(ctx context.Context, ownerUserID uint64, urlMatchKey string) (domainbrowserbookmark.BrowserBookmark, error) {
	if r.findFirstURLByMatchKeyFunc == nil {
		return domainbrowserbookmark.BrowserBookmark{}, repository.ErrNotFound
	}
	return r.findFirstURLByMatchKeyFunc(ownerUserID, urlMatchKey)
}

func (r *routerTestBrowserBookmarkRepository) NextSortOrder(ctx context.Context, ownerUserID uint64, parentID *uint64) (int, error) {
	if r.nextSortOrderFunc == nil {
		return 1000, nil
	}
	return r.nextSortOrderFunc(ownerUserID, parentID)
}

func (r *routerTestBrowserBookmarkRepository) Create(ctx context.Context, input repository.CreateBrowserBookmarkInput) (domainbrowserbookmark.BrowserBookmark, error) {
	if r.createFunc == nil {
		return domainbrowserbookmark.BrowserBookmark{}, nil
	}
	return r.createFunc(input)
}

func (r *routerTestBrowserBookmarkRepository) UpdateOwnerByID(ctx context.Context, id, ownerUserID uint64, input repository.UpdateBrowserBookmarkInput) (domainbrowserbookmark.BrowserBookmark, error) {
	return domainbrowserbookmark.BrowserBookmark{}, repository.ErrNotFound
}

func (r *routerTestBrowserBookmarkRepository) MoveOwnerByID(ctx context.Context, id, ownerUserID uint64, parentID *uint64) error {
	return nil
}

func (r *routerTestBrowserBookmarkRepository) UpdateSortOrders(ctx context.Context, ownerUserID uint64, orders []repository.BrowserBookmarkSortOrder) error {
	return nil
}

func (r *routerTestBrowserBookmarkRepository) SoftDeleteTreeOwnerByID(ctx context.Context, id, ownerUserID uint64) (bool, error) {
	if r.softDeleteTreeOwnerByIDFunc == nil {
		return false, nil
	}
	return r.softDeleteTreeOwnerByIDFunc(id, ownerUserID)
}

func newBookmarkBehaviorTestEngine(bookmarkRepo *routerTestBrowserBookmarkRepository) http.Handler {
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
	browserBookmarkHandler := httpHandler.NewBrowserBookmarkHandler(usecase.NewBrowserBookmarkUseCase(bookmarkRepo, nil, auditSink))
	browserFileMappingHandler := httpHandler.NewBrowserFileMappingHandler(nil)

	return New(cfg, logger, healthHandler, authHandler, userHandler, libraryHandler, nodeHandler, directoryHandler, fileHandler, tagHandler, browserBookmarkHandler, browserFileMappingHandler)
}
