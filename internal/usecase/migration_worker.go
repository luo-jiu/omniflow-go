package usecase

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"omniflow-go/internal/repository"
)

// 默认 worker 池大小：保守值，避免一次抢空 MinIO 带宽。
const defaultMigrationWorkerCount = 2

// migrationWorkerIdleSleep 抢不到 pending 子项时的轮询间隔。
const migrationWorkerIdleSleep = 5 * time.Second

// MigrationWorkerPool 管理一组迁移子项处理 worker。
type MigrationWorkerPool struct {
	uc      *MigrationUseCase
	stopCh  chan struct{}
	wg      sync.WaitGroup
	workers int
}

// NewMigrationWorkerPool 启动一组 worker 持续抢 pending 子项。
//
//	count<=0 时回退到默认 worker 数；
//	返回的 stop 函数关闭 stopCh 并等待所有 worker 退出，bootstrap cleanup 时调用。
func NewMigrationWorkerPool(uc *MigrationUseCase, count int) (*MigrationWorkerPool, func()) {
	if count <= 0 {
		count = defaultMigrationWorkerCount
	}
	pool := &MigrationWorkerPool{
		uc:      uc,
		stopCh:  make(chan struct{}),
		workers: count,
	}
	pool.start()
	return pool, pool.stop
}

func (p *MigrationWorkerPool) start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.runWorker(i)
	}
}

func (p *MigrationWorkerPool) stop() {
	close(p.stopCh)
	p.wg.Wait()
}

func (p *MigrationWorkerPool) runWorker(workerID int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.stopCh:
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		processed, err := p.tickOnce(ctx)
		cancel()
		if err != nil {
			slog.Warn("migration.worker.tick_error", "worker_id", workerID, "error", err)
		}
		if processed {
			// 抢到任务时立即继续，不睡，让吞吐量贴近 worker 数 × 处理速度。
			continue
		}

		select {
		case <-p.stopCh:
			return
		case <-time.After(migrationWorkerIdleSleep):
		}
	}
}

// tickOnce 抢一个 pending 子项并处理。
//
//	返回 (processed, error)：processed=true 表示处理了一个子项（无论成功失败）；
//	processed=false 表示当前没有可领的子项。
func (p *MigrationWorkerPool) tickOnce(ctx context.Context) (bool, error) {
	if p.uc == nil || p.uc.repo == nil {
		return false, errors.New("migration usecase not configured")
	}

	item, ok, err := p.uc.repo.ClaimNextItem(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if !ok {
		return false, nil
	}

	if err := p.uc.processItem(ctx, item); err != nil {
		// processItem 已写 audit / log / mark failed，这里返回让 runWorker 决定是否 sleep。
		// 仍然返回 processed=true，因为已经"动过"一个 item。
		return true, nil
	}
	return true, nil
}
