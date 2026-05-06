// Package uploadprogress 定义上传进度跟踪的端口与领域类型。
//
// 当前实现为单实例内存版本，专门用于 proxy 模式下让前端看到 backend → MinIO
// 这一段的真实进度。该端口刻意保持小而稳：未来切到客户端直传 MinIO 时，
// 整个 tracker 可降级为 no-op 实现，handler 与 UI 调用面零变动。
package uploadprogress

import (
	"context"
	"errors"
)

// State 表示一个上传会话的当前阶段。
type State string

const (
	// StateRunning 表示会话仍在累加字节。
	StateRunning State = "running"
	// StateDone 表示会话已结束并保留到 TTL 过期，便于前端最后一次轮询观察终态。
	StateDone State = "done"
)

// Progress 是单次查询返回的进度快照。
type Progress struct {
	UploadID      string
	TotalBytes    int64
	UploadedBytes int64
	Percentage    float64
	State         State
}

// Tracker 是上传进度跟踪端口。Register / Add / Done 由写链路调用，
// Get 由查询接口调用。actor 校验由实现内部完成，调用方传入当前请求的 actor ID。
type Tracker interface {
	// Register 在写链路开始前登记会话，total 为期望写入的总字节数；
	// actorID 来自当前请求 actor。已存在的同 ID 会话会被覆盖。
	Register(uploadID string, total int64, actorID string)

	// Add 累加 delta 字节到会话。uploadID 不存在时静默忽略，
	// 让 wrapProgressReader 在 uploadID 为空或会话已被清理时仍能透传。
	Add(uploadID string, delta int64)

	// Get 返回会话快照。uploadID 不存在或 actor 不匹配时返回 ErrNotFound，
	// 不区分两种情况以避免 uploadID 枚举泄漏。
	Get(ctx context.Context, uploadID string, actorID string) (Progress, error)

	// Done 标记会话进入终态并启动短 TTL 清理；幂等，重复调用安全。
	Done(uploadID string)
}

// ErrNotFound 表示 uploadID 不存在，或 actor 与会话不匹配。
// 调用方应统一映射为 HTTP 404。
var ErrNotFound = errors.New("upload progress: not found")
