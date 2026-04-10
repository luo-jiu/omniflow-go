package usecase

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"omniflow-go/internal/storage"
)

type UploadObjectCommand struct {
	Path        string
	FileName    string
	FileSize    int64
	ContentType string
	Content     io.Reader
}

type GetObjectLinkQuery struct {
	Path          string
	FileName      string
	ExpiryMinutes int
}

type FileUseCase struct {
	store storage.ObjectStorage
}

func NewFileUseCase(store storage.ObjectStorage) *FileUseCase {
	return &FileUseCase{store: store}
}

func (u *FileUseCase) UploadAndGetLink(ctx context.Context, cmd UploadObjectCommand) (string, error) {
	if err := u.ensureStoreConfigured(); err != nil {
		return "", err
	}
	if cmd.Content == nil {
		slog.WarnContext(ctx, "file.upload.invalid_argument", "reason", "content_missing")
		return "", fmt.Errorf("%w: file content is required", ErrInvalidArgument)
	}
	if cmd.FileSize < 0 {
		slog.WarnContext(ctx, "file.upload.invalid_argument", "reason", "file_size_lt_zero", "file_size", cmd.FileSize)
		return "", fmt.Errorf("%w: file size must be >= 0", ErrInvalidArgument)
	}

	objectName, err := buildObjectName(cmd.Path, cmd.FileName)
	if err != nil {
		return "", err
	}

	contentType := strings.TrimSpace(cmd.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := u.store.Upload(ctx, objectName, cmd.Content, cmd.FileSize, contentType); err != nil {
		return "", err
	}
	url, err := u.store.GetPresignedURL(ctx, objectName, 60*time.Minute)
	if err != nil {
		return "", err
	}
	slog.InfoContext(ctx, "file.upload.completed",
		"object_name", objectName,
		"file_size", cmd.FileSize,
		"content_type", contentType,
	)
	return url, nil
}

func (u *FileUseCase) GetFileLink(ctx context.Context, query GetObjectLinkQuery) (string, error) {
	if err := u.ensureStoreConfigured(); err != nil {
		return "", err
	}

	objectName, err := buildObjectName(query.Path, query.FileName)
	if err != nil {
		return "", err
	}

	expiry := query.ExpiryMinutes
	if expiry <= 0 {
		expiry = 60
	}
	url, err := u.store.GetPresignedURL(ctx, objectName, time.Duration(expiry)*time.Minute)
	if err != nil {
		return "", err
	}
	slog.DebugContext(ctx, "file.link.generated",
		"object_name", objectName,
		"expiry_minutes", expiry,
	)
	return url, nil
}

func buildObjectName(pathValue, fileName string) (string, error) {
	name := strings.TrimSpace(fileName)
	if name == "" {
		return "", fmt.Errorf("%w: file name is required", ErrInvalidArgument)
	}

	name = filepath.Base(name)
	pathValue = strings.Trim(strings.TrimSpace(pathValue), "/")
	if pathValue == "" {
		pathValue = "default"
	}
	return pathValue + "/" + name, nil
}

func (u *FileUseCase) ensureStoreConfigured() error {
	if u == nil || u.store == nil {
		return fmt.Errorf("%w: object storage not configured", ErrInvalidArgument)
	}
	return nil
}
