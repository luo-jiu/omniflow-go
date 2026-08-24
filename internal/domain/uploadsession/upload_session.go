package uploadsession

import "time"

// Mode 表示上传会话的模式：单端点 PUT 或 S3 multipart。
const (
	ModeSingle      = "single"
	ModeMultipart   = "multipart"
	StatusPending   = "pending"
	StatusCommitted = "committed"
)

// UploadSession 是直传 MinIO 流程的会话域模型。
// 后端在 init 阶段生成并落库，多次 sign/list/renew 期间存活；complete 后转为短期回执，abort 时删除。
type UploadSession struct {
	ID                string
	LibraryID         uint64
	ParentID          uint64
	ActorID           string
	StorageKey        string
	FileName          string
	FileSize          int64
	ContentType       string
	StorageProvider   string
	Mode              string
	MinioUploadID     string
	PartSize          int64
	Status            string
	ClientOperationID string
	CompletedNodeID   uint64
	CompletionResult  string
	CompletedAt       time.Time
	ExpiresAt         time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
