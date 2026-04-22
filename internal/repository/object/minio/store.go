package repository

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"omniflow-go/internal/config"
	"omniflow-go/internal/storage"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var _ storage.ObjectStorage = (*Store)(nil)

// Store 是 MinIO 的对象存储实现。
type Store struct {
	client *minio.Client
	core   minio.Core
	bucket string
}

func NewStore(cfg *config.Config) (storage.ObjectStorage, func(), error) {
	endpoint, secure, err := normalizeMinIOEndpoint(cfg.MinIO.Endpoint, cfg.MinIO.UseSSL)
	if err != nil {
		return nil, nil, err
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIO.AccessKey, cfg.MinIO.SecretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("new minio client: %w", err)
	}

	return &Store{
		client: client,
		core:   minio.Core{Client: client},
		bucket: cfg.MinIO.Bucket,
	}, func() {}, nil
}

func (s *Store) Bucket() string {
	return s.bucket
}

func (s *Store) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		select {
		case <-ctx.Done():
			return fmt.Errorf("check bucket exists: %w", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
		exists, err = s.client.BucketExists(ctx, s.bucket)
		if err != nil {
			return fmt.Errorf("check bucket exists: %w", err)
		}
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		if isBucketAlreadyExistsError(err) {
			return nil
		}
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

func (s *Store) InitiateMultipartUpload(
	ctx context.Context,
	objectName string,
	contentType string,
) (string, error) {
	if err := s.ensureBucket(ctx); err != nil {
		return "", err
	}
	uploadID, err := s.core.NewMultipartUpload(ctx, s.bucket, objectName, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("initiate multipart upload: %w", err)
	}
	return uploadID, nil
}

func (s *Store) UploadPart(
	ctx context.Context,
	objectName string,
	uploadID string,
	partNumber int,
	reader io.Reader,
	size int64,
) (string, error) {
	part, err := s.core.PutObjectPart(
		ctx, s.bucket, objectName, uploadID, partNumber,
		reader, size, minio.PutObjectPartOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("upload part %d: %w", partNumber, err)
	}
	return part.ETag, nil
}

func (s *Store) CompleteMultipartUpload(
	ctx context.Context,
	objectName string,
	uploadID string,
	parts []storage.MultipartUploadPart,
) error {
	completeParts := make([]minio.CompletePart, len(parts))
	for i, p := range parts {
		completeParts[i] = minio.CompletePart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		}
	}
	_, err := s.core.CompleteMultipartUpload(
		ctx, s.bucket, objectName, uploadID, completeParts, minio.PutObjectOptions{},
	)
	if err != nil {
		return fmt.Errorf("complete multipart upload: %w", err)
	}
	return nil
}

func (s *Store) AbortMultipartUpload(
	ctx context.Context,
	objectName string,
	uploadID string,
) error {
	err := s.core.AbortMultipartUpload(ctx, s.bucket, objectName, uploadID)
	if err != nil {
		return fmt.Errorf("abort multipart upload: %w", err)
	}
	return nil
}

func (s *Store) ListParts(
	ctx context.Context,
	objectName string,
	uploadID string,
) ([]storage.MultipartUploadPart, error) {
	var result []storage.MultipartUploadPart
	marker := 0
	for {
		resp, err := s.core.ListObjectParts(ctx, s.bucket, objectName, uploadID, marker, 1000)
		if err != nil {
			return nil, fmt.Errorf("list object parts: %w", err)
		}
		for _, p := range resp.ObjectParts {
			result = append(result, storage.MultipartUploadPart{
				PartNumber: p.PartNumber,
				ETag:       p.ETag,
				Size:       p.Size,
			})
		}
		if !resp.IsTruncated {
			break
		}
		marker = resp.NextPartNumberMarker
	}
	return result, nil
}

func normalizeMinIOEndpoint(raw string, useSSL bool) (string, bool, error) {
	endpoint := strings.TrimSpace(raw)
	if endpoint == "" {
		return "", useSSL, fmt.Errorf("minio endpoint is empty")
	}

	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return "", useSSL, fmt.Errorf("invalid minio endpoint: %w", err)
		}
		if parsed.Host == "" {
			return "", useSSL, fmt.Errorf("invalid minio endpoint: missing host")
		}
		endpoint = parsed.Host
		useSSL = parsed.Scheme == "https"
	}

	return endpoint, useSSL, nil
}

func isBucketAlreadyExistsError(err error) bool {
	resp := minio.ToErrorResponse(err)
	return resp.Code == "BucketAlreadyOwnedByYou" || resp.Code == "BucketAlreadyExists"
}
