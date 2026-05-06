package usecase

import (
	"io"

	"omniflow-go/internal/uploadprogress"
)

// wrapProgressReader 在底层 reader 之上叠加进度计数。
// 当 tracker 为 nil 或 uploadID 为空时透传，调用站点不必再判空。
func wrapProgressReader(r io.Reader, tracker uploadprogress.Tracker, uploadID string) io.Reader {
	if r == nil || tracker == nil || uploadID == "" {
		return r
	}
	return &progressReader{inner: r, tracker: tracker, uploadID: uploadID}
}

type progressReader struct {
	inner    io.Reader
	tracker  uploadprogress.Tracker
	uploadID string
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.inner.Read(buf)
	if n > 0 {
		p.tracker.Add(p.uploadID, int64(n))
	}
	return n, err
}
