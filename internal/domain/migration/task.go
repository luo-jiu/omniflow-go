// Package migration 定义存储迁移领域对象与状态常量。
//
// 迁移以 storage_object 为最小物理单位，所有引用同一 storage_object 的 node_files
// 在迁移完成时一起切换到新的 storage_object，绝不创建副本。任务由专用 worker pool
// 串行处理，单条 item 走 copy → verify → swap → delete 的 v1 即时删源策略。
package migration

import "time"

// 任务级状态。
const (
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusCanceled  = "canceled"
)

// 子项级状态。
const (
	ItemStatusPending = "pending"
	ItemStatusRunning = "running"
	ItemStatusDone    = "done"
	ItemStatusFailed  = "failed"
	ItemStatusSkipped = "skipped"
)

// Task 表示一次迁移任务的整体状态。
type Task struct {
	ID               string
	ActorID          string
	LibraryID        int64
	RootNodeID       int64
	TargetProvider   string
	Status           string
	TotalObjects     int32
	CompletedObjects int32
	FailedObjects    int32
	SkippedObjects   int32
	TotalBytes       int64
	TransferredBytes int64
	CurrentObjectKey string
	ErrorMessage     string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	StartedAt        *time.Time
	FinishedAt       *time.Time
}

// IsTerminal 判断任务是否处于终态。
func (t Task) IsTerminal() bool {
	switch t.Status {
	case TaskStatusCompleted, TaskStatusFailed, TaskStatusCanceled:
		return true
	default:
		return false
	}
}

// TaskItem 表示任务下的单个 storage_object 迁移子项。
type TaskItem struct {
	ID                    int64
	TaskID                string
	StorageObjectID       int64
	SourceProvider        string
	SourceBucket          string
	SourceKey             string
	TargetStorageObjectID int64
	TargetKey             string
	FileSize              int64
	Status                string
	ErrorMessage          string
	StartedAt             *time.Time
	FinishedAt            *time.Time
	CreatedAt             time.Time
}

// StorageObjectRef 是入队枚举返回的物理对象描述。
type StorageObjectRef struct {
	StorageObjectID int64
	Provider        string
	Bucket          string
	ObjectKey       string
	ContentLength   int64
	ContentType     string
}

// ProgressDelta 表示进度增量更新。
type ProgressDelta struct {
	CompletedObjects int32
	FailedObjects    int32
	SkippedObjects   int32
	TransferredBytes int64
	CurrentObjectKey string
}
