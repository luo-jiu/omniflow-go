package usecase

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	"omniflow-go/internal/authz"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/storage"

	"github.com/google/uuid"
)

type UploadFileCommand struct {
	Actor       actor.Actor
	LibraryID   uint64
	ParentID    uint64
	FileName    string
	FileSize    int64
	ContentType string
	Content     io.Reader
}

type GetFileLinkQuery struct {
	Actor     actor.Actor
	LibraryID uint64
	NodeID    uint64
	Expiry    time.Duration
}

type DirectoryUseCase struct {
	nodes      *NodeUseCase
	storage    storage.ObjectStorage
	authorizer authz.Authorizer
	auditLog   audit.Sink
}

func NewDirectoryUseCase(
	nodes *NodeUseCase,
	storage storage.ObjectStorage,
	authorizer authz.Authorizer,
	auditLog audit.Sink,
) *DirectoryUseCase {
	return &DirectoryUseCase{
		nodes:      nodes,
		storage:    storage,
		authorizer: authorizer,
		auditLog:   auditLog,
	}
}

func (u *DirectoryUseCase) UploadAndCreateNode(ctx context.Context, cmd UploadFileCommand) (domainnode.Node, error) {
	fileName := strings.TrimSpace(cmd.FileName)
	if cmd.LibraryID == 0 || fileName == "" || cmd.Content == nil {
		return domainnode.Node{}, fmt.Errorf("%w: library id, file name and content are required", ErrInvalidArgument)
	}
	if cmd.FileSize < 0 {
		return domainnode.Node{}, fmt.Errorf("%w: file size must be >= 0", ErrInvalidArgument)
	}
	if u.storage == nil {
		return domainnode.Node{}, fmt.Errorf("%w: object storage not configured", ErrInvalidArgument)
	}
	if err := u.authorize(ctx, cmd.Actor, cmd.LibraryID, authz.ActionUpload); err != nil {
		return domainnode.Node{}, err
	}

	base := filepath.Base(fileName)
	extWithDot := filepath.Ext(base)
	name := strings.TrimSuffix(base, extWithDot)
	if name == "" {
		name = base
	}
	ext := strings.TrimPrefix(extWithDot, ".")

	contentType := strings.TrimSpace(cmd.ContentType)
	if contentType == "" {
		contentType = mime.TypeByExtension(extWithDot)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	storageKey := fmt.Sprintf("libraries/%d/%s%s", cmd.LibraryID, uuid.NewString(), extWithDot)
	if err := u.storage.Upload(ctx, storageKey, cmd.Content, cmd.FileSize, contentType); err != nil {
		return domainnode.Node{}, err
	}

	node, err := u.nodes.Create(ctx, CreateNodeCommand{
		Actor:      cmd.Actor,
		Name:       name,
		Type:       domainnode.TypeFile,
		ParentID:   cmd.ParentID,
		LibraryID:  cmd.LibraryID,
		Ext:        ext,
		MIMEType:   contentType,
		FileSize:   cmd.FileSize,
		StorageKey: storageKey,
	})
	if err != nil {
		_ = u.storage.Delete(ctx, storageKey)
		return domainnode.Node{}, err
	}

	_ = u.RecordUploadIntent(ctx, cmd)
	_ = u.writeAudit(ctx, cmd.Actor, "directory.upload", true, map[string]any{
		"library_id":  cmd.LibraryID,
		"parent_id":   cmd.ParentID,
		"node_id":     node.ID,
		"name":        name,
		"storage_key": storageKey,
		"size":        cmd.FileSize,
		"mime_type":   contentType,
	})
	return node, nil
}

func (u *DirectoryUseCase) GetPresignedURL(ctx context.Context, query GetFileLinkQuery) (string, error) {
	if query.LibraryID == 0 || query.NodeID == 0 {
		return "", fmt.Errorf("%w: library id and node id are required", ErrInvalidArgument)
	}
	if u.storage == nil {
		return "", fmt.Errorf("%w: object storage not configured", ErrInvalidArgument)
	}
	if err := u.authorize(ctx, query.Actor, query.LibraryID, authz.ActionRead); err != nil {
		return "", err
	}

	node, err := u.nodes.findNodeView(ctx, query.NodeID, query.LibraryID)
	if err != nil {
		return "", err
	}
	if node.NodeType != nodeTypeFile {
		return "", fmt.Errorf("%w: node is not a file", ErrInvalidArgument)
	}
	if strings.TrimSpace(node.StorageKey) == "" {
		return "", ErrNotFound
	}

	expiry := query.Expiry
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	maxExpiry := 7 * 24 * time.Hour
	if expiry > maxExpiry {
		expiry = maxExpiry
	}

	url, err := u.storage.GetPresignedURL(ctx, node.StorageKey, expiry)
	if err != nil {
		return "", err
	}

	_ = u.writeAudit(ctx, query.Actor, "directory.presigned_url", true, map[string]any{
		"library_id": query.LibraryID,
		"node_id":    query.NodeID,
	})
	return url, nil
}

func (u *DirectoryUseCase) RecordUploadIntent(ctx context.Context, cmd UploadFileCommand) error {
	if u.auditLog == nil {
		return nil
	}

	return u.auditLog.Write(ctx, audit.Event{
		Actor:      cmd.Actor,
		Action:     "directory.upload.intent",
		Resource:   "library",
		Success:    true,
		OccurredAt: time.Now().UTC(),
		Metadata: map[string]any{
			"library_id": cmd.LibraryID,
			"parent_id":  cmd.ParentID,
			"file_name":  cmd.FileName,
		},
	})
}

func (u *DirectoryUseCase) authorize(ctx context.Context, principal actor.Actor, libraryID uint64, action authz.Action) error {
	if u.authorizer == nil {
		return nil
	}
	return u.authorizer.Authorize(ctx, principal, authz.Resource{
		Kind: "library",
		ID:   fmt.Sprintf("%d", libraryID),
	}, action)
}

func (u *DirectoryUseCase) writeAudit(ctx context.Context, principal actor.Actor, action string, success bool, metadata map[string]any) error {
	if u.auditLog == nil {
		return nil
	}
	return u.auditLog.Write(ctx, audit.Event{
		Actor:      principal,
		Action:     action,
		Resource:   "directory",
		Success:    success,
		OccurredAt: time.Now().UTC(),
		Metadata:   metadata,
	})
}
