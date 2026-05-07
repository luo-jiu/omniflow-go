package cli

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// uploadProgressPrinter 在 stderr 用 \r 原地刷新 part 完成进度。
// 每个 part 完成后由上传协程调用 add；最后由调用方调 finish 收尾。
// 80 ms 节流防终端刷新风暴；非 TTY 也能正常工作（每行 \r 而已）。
type uploadProgressPrinter struct {
	out        io.Writer
	totalBytes int64
	startedAt  time.Time

	mu         sync.Mutex
	uploaded   int64
	lastRender time.Time
	finished   bool
}

func newUploadProgressPrinter(out io.Writer, totalBytes int64) *uploadProgressPrinter {
	return &uploadProgressPrinter{
		out:        out,
		totalBytes: totalBytes,
		startedAt:  time.Now(),
	}
}

func (p *uploadProgressPrinter) add(n int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.finished {
		return
	}
	p.uploaded += n
	if p.uploaded > p.totalBytes {
		p.uploaded = p.totalBytes
	}
	if time.Since(p.lastRender) < 80*time.Millisecond {
		return
	}
	p.lastRender = time.Now()
	p.render(false)
}

func (p *uploadProgressPrinter) finish(success bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.finished {
		return
	}
	p.finished = true
	if success {
		p.uploaded = p.totalBytes
	}
	p.render(true)
	fmt.Fprintln(p.out)
}

// render 输出形如 "uploading: [#####     ] 50.2% 50.2/100.0 MiB 12.3 MiB/s"。
// 调用方持锁。
func (p *uploadProgressPrinter) render(_ bool) {
	const barWidth = 20
	pct := 0.0
	if p.totalBytes > 0 {
		pct = float64(p.uploaded) / float64(p.totalBytes) * 100
	}
	if pct > 100 {
		pct = 100
	}
	filled := min(int(pct*float64(barWidth)/100), barWidth)
	bar := make([]byte, barWidth)
	for i := range barWidth {
		if i < filled {
			bar[i] = '#'
		} else {
			bar[i] = ' '
		}
	}
	elapsed := time.Since(p.startedAt).Seconds()
	speed := 0.0
	if elapsed > 0 {
		speed = float64(p.uploaded) / elapsed
	}
	fmt.Fprintf(
		p.out,
		"\ruploading: [%s] %5.1f%% %s/%s %s/s",
		string(bar),
		pct,
		humanizeBytes(p.uploaded),
		humanizeBytes(p.totalBytes),
		humanizeBytes(int64(speed)),
	)
}

func humanizeBytes(n int64) string {
	const (
		KiB = 1024
		MiB = 1024 * KiB
		GiB = 1024 * MiB
	)
	switch {
	case n >= GiB:
		return fmt.Sprintf("%.2f GiB", float64(n)/float64(GiB))
	case n >= MiB:
		return fmt.Sprintf("%.2f MiB", float64(n)/float64(MiB))
	case n >= KiB:
		return fmt.Sprintf("%.2f KiB", float64(n)/float64(KiB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
