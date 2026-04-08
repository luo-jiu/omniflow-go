package repository

import (
	"context"
	"fmt"
	"io"
	"time"

	"omniflow-go/internal/config"
	"omniflow-go/internal/storage"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Store 是 MinIO 的对象存储实现。
type Store struct {
	client *minio.Client
	bucket string
}

func NewStore(cfg *config.Config) (storage.ObjectStorage, func(), error) {
	client, err := minio.New(cfg.MinIO.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIO.AccessKey, cfg.MinIO.SecretKey, ""),
		Secure: cfg.MinIO.UseSSL,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("new minio client: %w", err)
	}

	return &Store{
		client: client,
		bucket: cfg.MinIO.Bucket,
	}, func() {}, nil
}

func (s *Store) Bucket() string {
	return s.bucket
}

func (s *Store) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check bucket exists: %w", err)
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	return nil
}

func (s *Store) Upload(
	ctx context.Context,
	objectName string,
	reader io.Reader,
	size int64,
	contentType string,
) error {
	if err := s.ensureBucket(ctx); err != nil {
		return err
	}

	_, err := s.client.PutObject(ctx, s.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("upload object: %w", err)
	}
	return nil
}

func (s *Store) GetPresignedURL(
	ctx context.Context,
	objectName string,
	expiry time.Duration,
) (string, error) {
	if expiry <= 0 {
		expiry = 60 * time.Minute
	}

	url, err := s.client.PresignedGetObject(ctx, s.bucket, objectName, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("get presigned url: %w", err)
	}
	return url.String(), nil
}

func (s *Store) Delete(ctx context.Context, objectName string) error {
	err := s.client.RemoveObject(ctx, s.bucket, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}
