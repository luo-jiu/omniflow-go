package progress

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"omniflow-go/internal/uploadprogress"
)

func TestMemoryTracker_RegisterAddGet(t *testing.T) {
	tr, cleanup := NewMemoryTracker()
	defer cleanup()

	tr.Register("u1", 1000, "actor-1")
	tr.Add("u1", 250)
	tr.Add("u1", 250)

	p, err := tr.Get(context.Background(), "u1", "actor-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if p.UploadedBytes != 500 || p.TotalBytes != 1000 {
		t.Fatalf("unexpected progress: %+v", p)
	}
	if p.Percentage != 50 {
		t.Fatalf("expected 50%%, got %v", p.Percentage)
	}
	if p.State != uploadprogress.StateRunning {
		t.Fatalf("expected running, got %v", p.State)
	}
}

func TestMemoryTracker_GetNotFoundOnMissingOrActorMismatch(t *testing.T) {
	tr, cleanup := NewMemoryTracker()
	defer cleanup()

	tr.Register("u1", 100, "actor-1")

	if _, err := tr.Get(context.Background(), "missing", "actor-1"); !errors.Is(err, uploadprogress.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing id, got %v", err)
	}
	if _, err := tr.Get(context.Background(), "u1", "actor-2"); !errors.Is(err, uploadprogress.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for actor mismatch, got %v", err)
	}
}

func TestMemoryTracker_DonePinsToTotalAndStaysQueryable(t *testing.T) {
	tr, cleanup := NewMemoryTracker()
	defer cleanup()

	tr.Register("u1", 1000, "actor-1")
	tr.Add("u1", 900)
	tr.Done("u1")

	p, err := tr.Get(context.Background(), "u1", "actor-1")
	if err != nil {
		t.Fatalf("Get after Done failed: %v", err)
	}
	if p.UploadedBytes != 1000 {
		t.Fatalf("Done should pin uploaded to total, got %d", p.UploadedBytes)
	}
	if p.State != uploadprogress.StateDone {
		t.Fatalf("expected done state, got %v", p.State)
	}

	// Done 幂等
	tr.Done("u1")
	tr.Done("u1")
}

func TestMemoryTracker_EmptyUploadIDIsTransparent(t *testing.T) {
	tr, cleanup := NewMemoryTracker()
	defer cleanup()

	// 不应 panic，也不应留下任何条目
	tr.Register("", 100, "actor-1")
	tr.Add("", 10)
	tr.Done("")

	tr.mu.RLock()
	defer tr.mu.RUnlock()
	if len(tr.entries) != 0 {
		t.Fatalf("expected no entries, got %d", len(tr.entries))
	}
}

func TestMemoryTracker_ConcurrentAddIsSafe(t *testing.T) {
	tr, cleanup := NewMemoryTracker()
	defer cleanup()

	tr.Register("u1", 1_000_000, "actor-1")

	var wg sync.WaitGroup
	const goroutines = 50
	const perGoroutine = 100
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range perGoroutine {
				tr.Add("u1", 1)
			}
		}()
	}
	wg.Wait()

	p, err := tr.Get(context.Background(), "u1", "actor-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if p.UploadedBytes != int64(goroutines*perGoroutine) {
		t.Fatalf("expected %d, got %d", goroutines*perGoroutine, p.UploadedBytes)
	}
}

func TestMemoryTracker_SweepRemovesDoneAfterTTL(t *testing.T) {
	tr, cleanup := NewMemoryTracker()
	defer cleanup()

	base := time.Now()
	tr.now = func() time.Time { return base }

	tr.Register("u1", 100, "actor-1")
	tr.Done("u1")

	// 还在 TTL 内
	tr.now = func() time.Time { return base.Add(doneTTL - time.Second) }
	tr.sweep()
	if _, err := tr.Get(context.Background(), "u1", "actor-1"); err != nil {
		t.Fatalf("entry should still exist before TTL: %v", err)
	}

	// 越过 TTL 后被清理
	tr.now = func() time.Time { return base.Add(doneTTL + time.Second) }
	tr.sweep()
	if _, err := tr.Get(context.Background(), "u1", "actor-1"); !errors.Is(err, uploadprogress.ErrNotFound) {
		t.Fatalf("entry should be swept, got %v", err)
	}
}

func TestMemoryTracker_SweepRemovesZombieRunning(t *testing.T) {
	tr, cleanup := NewMemoryTracker()
	defer cleanup()

	base := time.Now()
	tr.now = func() time.Time { return base }
	tr.Register("u1", 100, "actor-1")

	tr.now = func() time.Time { return base.Add(doneTTL*maxRetainedRatio + time.Second) }
	tr.sweep()
	if _, err := tr.Get(context.Background(), "u1", "actor-1"); !errors.Is(err, uploadprogress.ErrNotFound) {
		t.Fatalf("zombie running entry should be swept, got %v", err)
	}
}
