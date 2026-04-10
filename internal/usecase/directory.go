package usecase

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	"omniflow-go/internal/authz"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/storage"

	"github.com/google/uuid"
	"github.com/samber/lo"
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

type BatchGetFileLinksQuery struct {
	Actor     actor.Actor
	LibraryID uint64
	NodeIDs   []uint64
	Expiry    time.Duration
}

type BatchFileLinkItem struct {
	NodeID uint64 `json:"nodeId"`
	URL    string `json:"url"`
}

type DirectoryUseCase struct {
	nodes      *NodeUseCase
	storage    storage.ObjectStorage
	authorizer authz.Authorizer
	auditLog   audit.Sink
}

const defaultUploadContentType = "application/octet-stream"

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
	fileName := cmd.FileName
	if cmd.LibraryID == 0 || strings.TrimSpace(fileName) == "" || cmd.Content == nil {
		slog.WarnContext(ctx, "directory.upload.invalid_argument",
			"library_id", cmd.LibraryID,
			"file_name", strings.TrimSpace(fileName),
			"has_content", cmd.Content != nil,
		)
		return domainnode.Node{}, fmt.Errorf("%w: library id, file name and content are required", ErrInvalidArgument)
	}
	if cmd.FileSize < 0 {
		slog.WarnContext(ctx, "directory.upload.invalid_argument",
			"library_id", cmd.LibraryID,
			"file_size", cmd.FileSize,
			"reason", "file_size_lt_zero",
		)
		return domainnode.Node{}, fmt.Errorf("%w: file size must be >= 0", ErrInvalidArgument)
	}
	if u.storage == nil {
		return domainnode.Node{}, fmt.Errorf("%w: object storage not configured", ErrInvalidArgument)
	}
	if err := u.authorize(ctx, cmd.Actor, cmd.LibraryID, authz.ActionUpload); err != nil {
		return domainnode.Node{}, err
	}

	base := extractUploadBaseName(fileName)
	extWithDot := path.Ext(base)
	name := strings.TrimSuffix(base, extWithDot)
	if name == "" {
		name = base
	}
	ext := strings.TrimPrefix(extWithDot, ".")

	contentReader, contentType, err := resolveUploadContentType(cmd.Content, cmd.ContentType, extWithDot)
	if err != nil {
		return domainnode.Node{}, fmt.Errorf("%w: resolve upload content type failed", ErrInvalidArgument)
	}

	storageKey := fmt.Sprintf("libraries/%d/%s%s", cmd.LibraryID, uuid.NewString(), extWithDot)
	if err := u.storage.Upload(ctx, storageKey, contentReader, cmd.FileSize, contentType); err != nil {
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
	slog.InfoContext(ctx, "directory.upload.completed",
		"library_id", cmd.LibraryID,
		"parent_id", cmd.ParentID,
		"node_id", node.ID,
		"size", cmd.FileSize,
	)
	return node, nil
}

func extractUploadBaseName(fileName string) string {
	normalized := strings.ReplaceAll(fileName, "\\", "/")
	base := path.Base(normalized)
	if base == "." || base == "/" {
		return fileName
	}
	return base
}

func resolveUploadContentType(
	content io.Reader,
	declaredContentType string,
	extWithDot string,
) (io.Reader, string, error) {
	contentType := strings.TrimSpace(declaredContentType)
	reader := content

	if contentType == "" || strings.EqualFold(contentType, defaultUploadContentType) {
		sniffed, replayReader, err := sniffContentType(content)
		if err != nil {
			return nil, "", err
		}
		reader = replayReader
		if sniffed != "" && !strings.EqualFold(sniffed, defaultUploadContentType) {
			contentType = sniffed
		}
	}

	if contentType == "" || strings.EqualFold(contentType, defaultUploadContentType) {
		byExt := strings.TrimSpace(mime.TypeByExtension(extWithDot))
		if byExt != "" {
			contentType = byExt
		}
	}

	if contentType == "" {
		contentType = defaultUploadContentType
	}
	return reader, contentType, nil
}

func sniffContentType(content io.Reader) (string, io.Reader, error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(content, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", nil, err
	}
	head = head[:n]
	replay := io.MultiReader(bytes.NewReader(head), content)
	if len(head) == 0 {
		return "", replay, nil
	}
	return http.DetectContentType(head), replay, nil
}

func (u *DirectoryUseCase) GetPresignedURL(ctx context.Context, query GetFileLinkQuery) (string, error) {
	if query.LibraryID == 0 || query.NodeID == 0 {
		slog.WarnContext(ctx, "directory.link.invalid_argument",
			"library_id", query.LibraryID,
			"node_id", query.NodeID,
		)
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
	if node.Type != domainnode.TypeFile {
		return "", fmt.Errorf("%w: node is not a file", ErrInvalidArgument)
	}
	if strings.TrimSpace(node.StorageKey) == "" {
		return "", ErrNotFound
	}

	expiry := query.Expiry
	if expiry <= 0 {
		expiry = 60 * time.Minute
	}

	url, err := u.storage.GetPresignedURL(ctx, node.StorageKey, expiry)
	if err != nil {
		return "", err
	}

	_ = u.writeAudit(ctx, query.Actor, "directory.presigned_url", true, map[string]any{
		"library_id": query.LibraryID,
		"node_id":    query.NodeID,
	})
	slog.DebugContext(ctx, "directory.link.generated",
		"library_id", query.LibraryID,
		"node_id", query.NodeID,
	)
	return url, nil
}

func (u *DirectoryUseCase) BatchGetPresignedURLs(
	ctx context.Context,
	query BatchGetFileLinksQuery,
) ([]BatchFileLinkItem, error) {
	if query.LibraryID == 0 {
		slog.WarnContext(ctx, "directory.links.batch.invalid_argument", "library_id", query.LibraryID)
		return []BatchFileLinkItem{}, fmt.Errorf("%w: library id is required", ErrInvalidArgument)
	}
	if len(query.NodeIDs) == 0 {
		return []BatchFileLinkItem{}, nil
	}
	if u.storage == nil {
		return []BatchFileLinkItem{}, fmt.Errorf("%w: object storage not configured", ErrInvalidArgument)
	}

	expiry := query.Expiry
	if expiry <= 0 {
		expiry = 60 * time.Minute
	}

	nodeIDs := normalizePositiveNodeIDs(query.NodeIDs)
	if len(nodeIDs) == 0 {
		return []BatchFileLinkItem{}, nil
	}

	storageKeys, err := u.nodes.ListFileStorageKeysByNodeIDs(ctx, query.Actor, query.LibraryID, nodeIDs)
	if err != nil {
		return nil, err
	}
	if len(storageKeys) == 0 {
		return []BatchFileLinkItem{}, nil
	}

	result := make([]BatchFileLinkItem, 0, len(storageKeys))
	for _, nodeID := range nodeIDs {
		storageKey := strings.TrimSpace(storageKeys[nodeID])
		if storageKey == "" {
			continue
		}
		url, urlErr := u.storage.GetPresignedURL(ctx, storageKey, expiry)
		if urlErr != nil {
			return nil, fmt.Errorf("generate presigned url for node %d: %w", nodeID, urlErr)
		}
		result = append(result, BatchFileLinkItem{
			NodeID: nodeID,
			URL:    url,
		})
	}
	slog.DebugContext(ctx, "directory.links.batch.generated",
		"library_id", query.LibraryID,
		"requested_count", len(nodeIDs),
		"result_count", len(result),
	)
	return result, nil
}

func normalizePositiveNodeIDs(nodeIDs []uint64) []uint64 {
	return lo.Uniq(lo.Filter(nodeIDs, func(nodeID uint64, _ int) bool {
		return nodeID > 0
	}))
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
