// Package progress 提供 uploadprogress.Tracker 的单实例内存实现。
//
// 设计前提：后端单实例部署、进度仅供前端轮询显示、不进入审计或持久化。
// 切到多实例或客户端直传时本包可整体废弃。
package progress

import (
	"context"
	"sync"
	"time"

	"omniflow-go/internal/uploadprogress"
)

// 默认参数：Done 后保留 doneTTL，让前端最后一次轮询能观察到 done 终态；
// janitor 周期低频，单实例假设下吞吐压力可忽略。
const (
	doneTTL          = 30 * time.Second
	janitorInterval  = 1 * time.Minute
	maxRetainedRatio = 4 // running 会话的最大保留时长 = doneTTL * maxRetainedRatio
)

type entry struct {
	total      int64
	uploaded   int64
	actorID    string
	state      uploadprogress.State
	updatedAt  time.Time
	doneAt     time.Time // 仅在 state == StateDone 时有效
	registered time.Time
}

// MemoryTracker 是 uploadprogress.Tracker 的内存实现，
// 用 RWMutex 保护一个 map，并启动一个后台 janitor 清理过期条目。
type MemoryTracker struct {
	mu      sync.RWMutex
	entries map[string]*entry
	now     func() time.Time

	stopCh chan struct{}
	stopWg sync.WaitGroup
}

// NewMemoryTracker 构造并启动一个内存版 Tracker。返回的 cleanup 函数停止
// 后台 janitor，调用方有责任在程序退出时调用一次。
func NewMemoryTracker() (*MemoryTracker, func()) {
	t := &MemoryTracker{
		entries: make(map[string]*entry),
		now:     time.Now,
		stopCh:  make(chan struct{}),
	}
	t.stopWg.Add(1)
	go t.janitor()
	return t, func() {
		close(t.stopCh)
		t.stopWg.Wait()
	}
}

// Register 登记新会话。若 uploadID 为空则忽略，便于上游使用方在未启用进度跟踪
// 时保持调用站点不变。
func (t *MemoryTracker) Register(uploadID string, total int64, actorID string) {
	if uploadID == "" {
		return
	}
	now := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries[uploadID] = &entry{
		total:      total,
		actorID:    actorID,
		state:      uploadprogress.StateRunning,
		updatedAt:  now,
		registered: now,
	}
}

// Add 累加字节。uploadID 为空或会话已被清理时静默退出，确保 ProgressReader
// 在退化为透传时不会触发 panic。
func (t *MemoryTracker) Add(uploadID string, delta int64) {
	if uploadID == "" || delta <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entries[uploadID]
	if !ok {
		return
	}
	e.uploaded += delta
	e.updatedAt = t.now()
}

// Get 返回会话快照。
func (t *MemoryTracker) Get(_ context.Context, uploadID string, actorID string) (uploadprogress.Progress, error) {
	if uploadID == "" {
		return uploadprogress.Progress{}, uploadprogress.ErrNotFound
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	e, ok := t.entries[uploadID]
	if !ok || e.actorID != actorID {
		return uploadprogress.Progress{}, uploadprogress.ErrNotFound
	}
	percentage := 0.0
	if e.total > 0 {
		ratio := float64(e.uploaded) / float64(e.total)
		if ratio > 1 {
			ratio = 1
		}
		percentage = ratio * 100
	}
	return uploadprogress.Progress{
		UploadID:      uploadID,
		TotalBytes:    e.total,
		UploadedBytes: e.uploaded,
		Percentage:    percentage,
		State:         e.state,
	}, nil
}

// Done 标记会话终态，doneTTL 后由 janitor 清理。
func (t *MemoryTracker) Done(uploadID string) {
	if uploadID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entries[uploadID]
	if !ok {
		return
	}
	now := t.now()
	if e.state != uploadprogress.StateDone {
		e.state = uploadprogress.StateDone
		e.doneAt = now
		// 终态时把 uploaded 拉齐到 total，避免最后一次轮询因尾部累加未到达
		// 而显示 99%。total 为 0 的边界情形也能拿到 0%。
		if e.total > 0 && e.uploaded < e.total {
			e.uploaded = e.total
		}
		e.updatedAt = now
	}
}

func (t *MemoryTracker) janitor() {
	defer t.stopWg.Done()
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.sweep()
		}
	}
}

func (t *MemoryTracker) sweep() {
	now := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, e := range t.entries {
		switch e.state {
		case uploadprogress.StateDone:
			if now.Sub(e.doneAt) >= doneTTL {
				delete(t.entries, id)
			}
		default:
			// 防御性清理：长时间没收到 Done 也没继续累加的 running 会话视为僵尸。
			if now.Sub(e.updatedAt) >= doneTTL*maxRetainedRatio {
				delete(t.entries, id)
			}
		}
	}
}
