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

type ObjectStorage interface {
	Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error
	GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, objectName string) error
	Bucket() string

	// InitiateMultipartUpload 创建分片上传会话，返回 MinIO 的 uploadID。
	InitiateMultipartUpload(ctx context.Context, objectName string, contentType string) (uploadID string, err error)
	// UploadPart 上传单个分片，返回 ETag。
	UploadPart(ctx context.Context, objectName string, uploadID string, partNumber int, reader io.Reader, size int64) (etag string, err error)
	// CompleteMultipartUpload 合并所有分片，完成上传。
	CompleteMultipartUpload(ctx context.Context, objectName string, uploadID string, parts []MultipartUploadPart) error
	// AbortMultipartUpload 取消分片上传，清理已上传分片。
	AbortMultipartUpload(ctx context.Context, objectName string, uploadID string) error
	// ListParts 列出已上传的分片。
	ListParts(ctx context.Context, objectName string, uploadID string) ([]MultipartUploadPart, error)
}
