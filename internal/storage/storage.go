package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrProviderNotImplemented = errors.New("object storage provider is not implemented")
	ErrProviderUnknown        = errors.New("unknown object storage provider")
)

// MultipartUploadPart 表示分片上传中的单个分片信息。
type MultipartUploadPart struct {
	PartNumber int
	ETag       string
	Size       int64
}

// ObjectInfo 表示对象的基础元信息，用于直传完成校验。
type ObjectInfo struct {
	Size        int64
	ContentType string
	ETag        string
}

type ObjectStorage interface {
	Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error
	GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, objectName string) error
	Bucket() string

	// PresignedPutObject 颁发单端点 PUT 直传的预签名 URL。
	PresignedPutObject(ctx context.Context, objectName string, expiry time.Duration) (string, error)
	// StatObject 在直传 single 模式 complete 阶段校验对象是否真实写入。
	StatObject(ctx context.Context, objectName string) (ObjectInfo, error)
	// GetObject 流式读对象，调用方负责关闭 reader。用于跨 provider 物理迁移。
	GetObject(ctx context.Context, objectName string) (io.ReadCloser, ObjectInfo, error)

	// InitiateMultipartUpload 创建分片上传会话，返回 MinIO 的 uploadID。
	InitiateMultipartUpload(ctx context.Context, objectName string, contentType string) (uploadID string, err error)
	// UploadPart 上传单个分片，返回 ETag。
	UploadPart(ctx context.Context, objectName string, uploadID string, partNumber int, reader io.Reader, size int64) (etag string, err error)
	// PresignedUploadPart 颁发分片 PUT 的预签名 URL（携带 uploadId 与 partNumber）。
	PresignedUploadPart(ctx context.Context, objectName string, uploadID string, partNumber int, expiry time.Duration) (string, error)
	// CompleteMultipartUpload 合并所有分片，完成上传。
	CompleteMultipartUpload(ctx context.Context, objectName string, uploadID string, parts []MultipartUploadPart) error
	// AbortMultipartUpload 取消分片上传，清理已上传分片。
	AbortMultipartUpload(ctx context.Context, objectName string, uploadID string) error
	// ListParts 列出已上传的分片。
	ListParts(ctx context.Context, objectName string, uploadID string) ([]MultipartUploadPart, error)
}
