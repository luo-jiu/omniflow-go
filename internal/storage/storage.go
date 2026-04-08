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

type ObjectStorage interface {
	Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error
	GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, objectName string) error
	Bucket() string
}
