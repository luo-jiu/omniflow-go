package usecase

import (
	"context"
	"fmt"
	"io"
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
	if u.store == nil {
		return "", fmt.Errorf("%w: object storage not configured", ErrInvalidArgument)
	}
	if cmd.Content == nil {
		return "", fmt.Errorf("%w: file content is required", ErrInvalidArgument)
	}
	if cmd.FileSize < 0 {
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
	return u.store.GetPresignedURL(ctx, objectName, 60*time.Minute)
}

func (u *FileUseCase) GetFileLink(ctx context.Context, query GetObjectLinkQuery) (string, error) {
	if u.store == nil {
		return "", fmt.Errorf("%w: object storage not configured", ErrInvalidArgument)
	}

	objectName, err := buildObjectName(query.Path, query.FileName)
	if err != nil {
		return "", err
	}

	expiry := query.ExpiryMinutes
	if expiry <= 0 {
		expiry = 60
	}
	return u.store.GetPresignedURL(ctx, objectName, time.Duration(expiry)*time.Minute)
}

func buildObjectName(pathValue, fileName string) (string, error) {
	name := strings.TrimSpace(fileName)
	if name == "" {
		return "", fmt.Errorf("%w: file name is required", ErrInvalidArgument)
	}

	name = filepath.Base(name)
	pathValue = strings.TrimSpace(pathValue)
	if pathValue == "" {
		pathValue = "default"
	}
	pathValue = strings.Trim(pathValue, "/")
	if pathValue == "" {
		pathValue = "default"
	}
	return pathValue + "/" + name, nil
}
