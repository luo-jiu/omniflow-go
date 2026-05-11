package usecase

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	"omniflow-go/internal/authz"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/repository"
	"omniflow-go/internal/storage"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

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

type UpdateFileContentCommand struct {
	Actor           actor.Actor
	LibraryID       uint64
	NodeID          uint64
	FileSize        int64
	ContentType     string
	StorageProvider string
	Content         io.Reader
	DryRun          bool
}

type DirectoryUseCase struct {
	nodes      *NodeUseCase
	registry   *storage.StorageRegistry
	authorizer authz.Authorizer
	auditLog   audit.Sink
}

const defaultUploadContentType = "application/octet-stream"

func NewDirectoryUseCase(
	nodes *NodeUseCase,
	registry *storage.StorageRegistry,
	authorizer authz.Authorizer,
	auditLog audit.Sink,
) *DirectoryUseCase {
	return &DirectoryUseCase{
		nodes:      nodes,
		registry:   registry,
		authorizer: authorizer,
		auditLog:   auditLog,
	}
}

func extractUploadBaseName(fileName string) string {
	normalized := strings.ReplaceAll(fileName, "\\", "/")
	base := path.Base(normalized)
	if base == "." || base == "/" {
		return fileName
	}
	return base
}

func (u *DirectoryUseCase) GetPresignedURL(ctx context.Context, query GetFileLinkQuery) (string, error) {
	if query.LibraryID == 0 || query.NodeID == 0 {
		slog.WarnContext(ctx, "directory.link.invalid_argument",
			"library_id", query.LibraryID,
			"node_id", query.NodeID,
		)
		return "", fmt.Errorf("%w: library id and node id are required", ErrInvalidArgument)
	}
	if u.registry == nil {
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

	providerAlias, providerErr := u.nodes.GetFileStorageProvider(ctx, query.NodeID, query.LibraryID)
	if providerErr != nil {
		return "", fmt.Errorf("resolve storage provider for node %d: %w", query.NodeID, providerErr)
	}
	store, storeErr := u.registry.Get(providerAlias)
	if storeErr != nil {
		return "", fmt.Errorf("storage provider %q not available: %w", providerAlias, storeErr)
	}

	expiry := query.Expiry
	if expiry <= 0 {
		expiry = 60 * time.Minute
	}

	url, err := store.GetPresignedURL(ctx, node.StorageKey, expiry)
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
	if u.registry == nil {
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

	storageInfos, err := u.nodes.ListFileStorageInfo(ctx, query.Actor, query.LibraryID, nodeIDs)
	if err != nil {
		return nil, err
	}
	if len(storageInfos) == 0 {
		return []BatchFileLinkItem{}, nil
	}

	result := make([]BatchFileLinkItem, 0, len(storageInfos))
	for _, info := range storageInfos {
		if strings.TrimSpace(info.StorageKey) == "" {
			continue
		}
		store, storeErr := u.registry.Get(info.ProviderAlias)
		if storeErr != nil {
			slog.WarnContext(ctx, "directory.links.batch.provider_unavailable",
				"node_id", info.NodeID,
				"provider", info.ProviderAlias,
				"error", storeErr,
			)
			continue
		}
		url, urlErr := store.GetPresignedURL(ctx, info.StorageKey, expiry)
		if urlErr != nil {
			slog.WarnContext(ctx, "directory.links.batch.presigned_url_failed",
				"node_id", info.NodeID,
				"provider", info.ProviderAlias,
				"error", urlErr,
			)
			continue
		}
		result = append(result, BatchFileLinkItem{
			NodeID: info.NodeID,
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

// UpdateFileContent 按节点 ID 原地替换文件内容，保留节点身份和目录位置。
func (u *DirectoryUseCase) UpdateFileContent(
	ctx context.Context,
	cmd UpdateFileContentCommand,
) (domainnode.Node, error) {
	if cmd.LibraryID == 0 || cmd.NodeID == 0 || cmd.Content == nil {
		return domainnode.Node{}, fmt.Errorf("%w: library id, node id and content are required", ErrInvalidArgument)
	}
	if cmd.FileSize < 0 {
		return domainnode.Node{}, fmt.Errorf("%w: file size must be >= 0", ErrInvalidArgument)
	}
	if u.registry == nil {
		return domainnode.Node{}, fmt.Errorf("%w: object storage not configured", ErrInvalidArgument)
	}
	if u.nodes == nil {
		return domainnode.Node{}, errNodeRepositoryNotConfigured
	}

	node, err := u.nodes.GetNodeDetail(ctx, cmd.Actor, cmd.NodeID)
	if err != nil {
		return domainnode.Node{}, err
	}
	if node.LibraryID != cmd.LibraryID {
		return domainnode.Node{}, fmt.Errorf(
			"%w: node %d is not in library %d",
			ErrInvalidArgument,
			cmd.NodeID,
			cmd.LibraryID,
		)
	}
	if node.Type != domainnode.TypeFile {
		return domainnode.Node{}, fmt.Errorf("%w: node is not a file", ErrInvalidArgument)
	}
	if err := u.nodes.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return domainnode.Node{}, err
	}

	extWithDot := ""
	if ext := strings.TrimSpace(node.Ext); ext != "" {
		extWithDot = "." + strings.TrimPrefix(ext, ".")
	}
	contentType := resolveDirectUploadContentType(cmd.ContentType, extWithDot)

	var store storage.ObjectStorage
	var providerAlias string
	if strings.TrimSpace(node.StorageKey) == "" {
		if overrideProvider := strings.TrimSpace(cmd.StorageProvider); overrideProvider != "" {
			store, err = u.registry.Get(overrideProvider)
			if err != nil {
				return domainnode.Node{}, fmt.Errorf(
					"%w: storage provider %q: %v",
					ErrInvalidArgument,
					overrideProvider,
					err,
				)
			}
			providerAlias = overrideProvider
			if _, canonicalAlias, ok := u.registry.ProviderConfigByAlias(overrideProvider); ok {
				providerAlias = canonicalAlias
			}
		} else {
			ext := strings.TrimPrefix(extWithDot, ".")
			store, providerAlias, err = u.registry.Resolve(cmd.FileSize, ext, contentType)
			if err != nil {
				return domainnode.Node{}, err
			}
		}
	} else {
		providerAlias, err = u.nodes.GetFileStorageProvider(ctx, cmd.NodeID, cmd.LibraryID)
		if err != nil {
			return domainnode.Node{}, fmt.Errorf("resolve storage provider for node %d: %w", cmd.NodeID, err)
		}
		store, err = u.registry.Get(providerAlias)
		if err != nil {
			return domainnode.Node{}, fmt.Errorf("storage provider %q not available: %w", providerAlias, err)
		}
	}

	if cmd.DryRun {
		_ = u.writeAudit(ctx, cmd.Actor, "directory.file_content.update", true, map[string]any{
			"library_id": cmd.LibraryID,
			"node_id":    cmd.NodeID,
			"size":       cmd.FileSize,
			"mime_type":  contentType,
			"mode":       resolveMutationMode(cmd.DryRun),
			"dry_run":    cmd.DryRun,
		})
		return node, nil
	}

	newStorageKey := fmt.Sprintf("libraries/%d/%s%s", cmd.LibraryID, uuid.NewString(), extWithDot)
	if err := store.Upload(ctx, newStorageKey, cmd.Content, cmd.FileSize, contentType); err != nil {
		return domainnode.Node{}, err
	}

	oldKey, err := u.nodes.ReplaceFileStorage(ctx, cmd.NodeID, cmd.LibraryID, repository.ReplaceFileStorageInput{
		NewObjectKey:   newStorageKey,
		NewFileSize:    cmd.FileSize,
		NewContentType: contentType,
		NewProvider:    providerAlias,
		NewBucket:      store.Bucket(),
	})
	if err != nil {
		_ = store.Delete(ctx, newStorageKey)
		return domainnode.Node{}, fmt.Errorf("replace file storage: %w", err)
	}

	if oldKey != "" && oldKey != newStorageKey {
		_ = store.Delete(ctx, oldKey)
	}

	updated, err := u.nodes.findNodeView(ctx, cmd.NodeID, cmd.LibraryID)
	if err != nil {
		return domainnode.Node{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "directory.file_content.update", true, map[string]any{
		"library_id":      cmd.LibraryID,
		"node_id":         cmd.NodeID,
		"new_storage_key": newStorageKey,
		"old_storage_key": oldKey,
		"size":            cmd.FileSize,
		"mime_type":       contentType,
		"mode":            resolveMutationMode(cmd.DryRun),
		"dry_run":         cmd.DryRun,
	})
	slog.InfoContext(ctx, "directory.file_content.update.completed",
		"library_id", cmd.LibraryID,
		"node_id", cmd.NodeID,
		"size", cmd.FileSize,
	)
	return updated, nil
}

func normalizePositiveNodeIDs(nodeIDs []uint64) []uint64 {
	return lo.Uniq(lo.Filter(nodeIDs, func(nodeID uint64, _ int) bool {
		return nodeID > 0
	}))
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
