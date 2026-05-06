package repository

import (
	progressrepo "omniflow-go/internal/repository/progress"
	"omniflow-go/internal/uploadprogress"
)

// NewUploadProgressTracker 构造一个内存版上传进度跟踪器。
// 切到多实例或客户端直传时只需替换实现，不影响调用方。
func NewUploadProgressTracker() (uploadprogress.Tracker, func()) {
	tracker, cleanup := progressrepo.NewMemoryTracker()
	return tracker, cleanup
}
