package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/transport/http/middleware"
	"omniflow-go/internal/uploadprogress"

	"github.com/gin-gonic/gin"
)

type fakeProgressTracker struct {
	getFunc func(uploadID, actorID string) (uploadprogress.Progress, error)
}

func (f *fakeProgressTracker) Register(string, int64, string) {}
func (f *fakeProgressTracker) Add(string, int64)              {}
func (f *fakeProgressTracker) Done(string)                    {}
func (f *fakeProgressTracker) Get(_ context.Context, uploadID, actorID string) (uploadprogress.Progress, error) {
	return f.getFunc(uploadID, actorID)
}

func newProgressContext(rec *httptest.ResponseRecorder, uploadID string, act actor.Actor) *gin.Context {
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/upload/"+uploadID+"/progress", nil)
	ctx.Params = gin.Params{{Key: "uploadId", Value: uploadID}}
	ctx.Set(middleware.ActorKey, act)
	return ctx
}

func TestUploadProgressHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tracker := &fakeProgressTracker{
		getFunc: func(uploadID, actorID string) (uploadprogress.Progress, error) {
			if uploadID != "u1" || actorID != "actor-1" {
				t.Fatalf("unexpected args: uploadID=%s actorID=%s", uploadID, actorID)
			}
			return uploadprogress.Progress{
				UploadID:      "u1",
				TotalBytes:    1000,
				UploadedBytes: 250,
				Percentage:    25,
				State:         uploadprogress.StateRunning,
			}, nil
		},
	}
	h := NewUploadProgressHandler(tracker)

	rec := httptest.NewRecorder()
	ctx := newProgressContext(rec, "u1", actor.Actor{ID: "actor-1", Kind: actor.KindUser})
	h.GetProgress(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp Result
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", resp.Data)
	}
	if data["uploadId"] != "u1" {
		t.Fatalf("expected uploadId=u1, got %v", data["uploadId"])
	}
	if data["state"] != "running" {
		t.Fatalf("expected state=running, got %v", data["state"])
	}
}

func TestUploadProgressHandler_NotFoundUniformResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tracker := &fakeProgressTracker{
		getFunc: func(string, string) (uploadprogress.Progress, error) {
			return uploadprogress.Progress{}, uploadprogress.ErrNotFound
		},
	}
	h := NewUploadProgressHandler(tracker)

	rec := httptest.NewRecorder()
	ctx := newProgressContext(rec, "u-missing", actor.Actor{ID: "actor-1", Kind: actor.KindUser})
	h.GetProgress(ctx)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for not-found, got %d", rec.Code)
	}
}

func TestUploadProgressHandler_MissingUploadID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUploadProgressHandler(&fakeProgressTracker{})

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/upload//progress", nil)
	// no params, simulate empty uploadId
	ctx.Set(middleware.ActorKey, actor.Actor{ID: "actor-1", Kind: actor.KindUser})

	h.GetProgress(ctx)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing uploadId, got %d", rec.Code)
	}
}

func TestUploadProgressHandler_NilTracker(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUploadProgressHandler(nil)

	rec := httptest.NewRecorder()
	ctx := newProgressContext(rec, "u1", actor.Actor{ID: "actor-1", Kind: actor.KindUser})
	h.GetProgress(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when tracker is nil, got %d", rec.Code)
	}
}
