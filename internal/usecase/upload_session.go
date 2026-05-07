package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"path"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	"omniflow-go/internal/authz"
	"omniflow-go/internal/config"
	domainnode "omniflow-go/internal/domain/node"
	domainsession "omniflow-go/internal/domain/uploadsession"
	"omniflow-go/internal/repository"
	uploadsessionpg "omniflow-go/internal/repository/postgres/impl/uploadsession"
	"omniflow-go/internal/storage"

	"github.com/google/uuid"
)

// 直传 single 模式阈值：≤16 MiB 走单端点 PUT；> 阈值才走 multipart。
// 选 16 MiB 是因为 S3 multipart 单 part 最小 5 MiB，16 MiB 给前端留 1 个 part 的空间又不至于让小文件多走一次 InitiateMultipart。
const directUploadSingleThresholdBytes int64 = 16 * 1024 * 1024

// 默认分片大小（multipart 模式），覆盖 cfg.Upload.ChunkSizeBytes 的下限。
const directUploadDefaultPartSizeBytes int64 = 16 * 1024 * 1024

// presigned URL 默认 1h；与会话 lease（24h）解耦。
const presignedURLDefaultExpiry = time.Hour

// InitUploadSessionCommand 创建直传会话所需参数。
type InitUploadSessionCommand struct {
	Actor           actor.Actor
	LibraryID       uint64
	ParentID        uint64
	FileName        string
	FileSize        int64
	ContentType     string
	StorageProvider string
}

// InitUploadSessionResult 是 init 阶段返回给客户端的会话元信息。
// PartSize 仅在 multipart 模式下有意义；single 模式 PartSize == FileSize。
type InitUploadSessionResult struct {
	UploadID   string    `json:"uploadId"`
	StorageKey string    `json:"storageKey"`
	Mode       string    `json:"mode"`
	PartSize   int64     `json:"partSize"`
	TotalParts int       `json:"totalParts"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

// SignUploadPartsCommand 颁发分片预签名 URL 所需参数。
type SignUploadPartsCommand struct {
	Actor       actor.Actor
	UploadID    string
	PartNumbers []int
	Expiry      time.Duration
}

// SignedUploadPart 单个分片的预签名 URL。
type SignedUploadPart struct {
	PartNumber int       `json:"partNumber"`
	URL        string    `json:"url"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

// SignUploadPartsResult 颁发结果 + 顺手刷新的会话 lease。
type SignUploadPartsResult struct {
	Parts     []SignedUploadPart `json:"parts"`
	ExpiresAt time.Time          `json:"expiresAt"`
}

// CompleteUploadSessionCommand 完成上传所需参数。
// Parts 仅 multipart 模式必填；single 模式忽略。
// ConflictPolicy 在 Complete 阶段决定同名节点处理策略；空串走 NodeUseCase.Create 默认（error）。
type CompleteUploadSessionCommand struct {
	Actor          actor.Actor
	UploadID       string
	Parts          []CompletedPart
	ConflictPolicy NodeNameConflictPolicy
}

// CompletedPart 客户端在 complete 阶段提交的分片清单条目。
type CompletedPart struct {
	PartNumber int
	ETag       string
}

// UploadSessionPart 透传 ListParts 的单条分片信息。
type UploadSessionPart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

// UploadSessionUseCase 编排直传 MinIO 全链路：init → sign/list/renew → complete/abort。
// 状态持久化在 Postgres upload_sessions 表，多实例就绪、崩溃可恢复。
type UploadSessionUseCase struct {
	repo       *repository.UploadSessionRepository
	nodes      *NodeUseCase
	registry   *storage.StorageRegistry
	authorizer authz.Authorizer
	auditLog   audit.Sink
	cfg        *config.Config
	stopCh     chan struct{}
}

// NewUploadSessionUseCase 仅构造，不启动 janitor。bootstrap 调 NewUploadSessionUseCaseWithJanitor。
func NewUploadSessionUseCase(
	repo *repository.UploadSessionRepository,
	nodes *NodeUseCase,
	registry *storage.StorageRegistry,
	authorizer authz.Authorizer,
	auditLog audit.Sink,
	cfg *config.Config,
) *UploadSessionUseCase {
	return &UploadSessionUseCase{
		repo:       repo,
		nodes:      nodes,
		registry:   registry,
		authorizer: authorizer,
		auditLog:   auditLog,
		cfg:        cfg,
		stopCh:     make(chan struct{}),
	}
}

// NewUploadSessionUseCaseWithJanitor 构造 UC 并启动后台 janitor，返回 stop 钩子。
func NewUploadSessionUseCaseWithJanitor(
	repo *repository.UploadSessionRepository,
	nodes *NodeUseCase,
	registry *storage.StorageRegistry,
	authorizer authz.Authorizer,
	auditLog audit.Sink,
	cfg *config.Config,
) (*UploadSessionUseCase, func()) {
	uc := NewUploadSessionUseCase(repo, nodes, registry, authorizer, auditLog, cfg)
	go uc.runJanitor()
	return uc, func() {
		close(uc.stopCh)
	}
}

// Init 创建会话：权限校验 → 解析 contentType / provider → 生成 storageKey → 模式分流 → 落库。
func (u *UploadSessionUseCase) Init(ctx context.Context, cmd InitUploadSessionCommand) (InitUploadSessionResult, error) {
	if u.repo == nil {
		return InitUploadSessionResult{}, fmt.Errorf("%w: upload session repository not configured", ErrInvalidArgument)
	}
	if u.registry == nil {
		return InitUploadSessionResult{}, fmt.Errorf("%w: object storage not configured", ErrInvalidArgument)
	}
	fileName := strings.TrimSpace(cmd.FileName)
	if cmd.LibraryID == 0 || fileName == "" {
		return InitUploadSessionResult{}, fmt.Errorf("%w: library id and file name are required", ErrInvalidArgument)
	}
	if cmd.FileSize <= 0 {
		return InitUploadSessionResult{}, fmt.Errorf("%w: file size must be > 0", ErrInvalidArgument)
	}
	if err := u.authorize(ctx, cmd.Actor, cmd.LibraryID, authz.ActionUpload); err != nil {
		return InitUploadSessionResult{}, err
	}

	base := extractUploadBaseName(fileName)
	extWithDot := path.Ext(base)
	ext := strings.TrimPrefix(extWithDot, ".")
	contentType := resolveDirectUploadContentType(cmd.ContentType, extWithDot)

	store, providerAlias, err := u.resolveProvider(cmd.StorageProvider, cmd.FileSize, ext, contentType)
	if err != nil {
		return InitUploadSessionResult{}, err
	}

	storageKey := fmt.Sprintf("libraries/%d/%s%s", cmd.LibraryID, uuid.NewString(), extWithDot)
	mode, partSize, totalParts := pickUploadMode(cmd.FileSize, u.cfg.Upload.ChunkSizeBytes)

	var minioUploadID string
	if mode == domainsession.ModeMultipart {
		minioUploadID, err = store.InitiateMultipartUpload(ctx, storageKey, contentType)
		if err != nil {
			return InitUploadSessionResult{}, fmt.Errorf("init multipart: %w", err)
		}
	}

	now := time.Now().UTC()
	expiresAt := now.Add(u.sessionTTL())
	sessionID := uuid.NewString()

	_, err = u.repo.Create(ctx, uploadsessionpg.CreateInput{
		ID:              sessionID,
		LibraryID:       cmd.LibraryID,
		ParentID:        cmd.ParentID,
		ActorID:         cmd.Actor.ID,
		StorageKey:      storageKey,
		FileName:        fileName,
		FileSize:        cmd.FileSize,
		ContentType:     contentType,
		StorageProvider: providerAlias,
		Mode:            mode,
		MinioUploadID:   minioUploadID,
		PartSize:        partSize,
		ExpiresAt:       expiresAt,
	})
	if err != nil {
		// 落库失败要回滚 MinIO multipart，避免悬挂会话。
		if mode == domainsession.ModeMultipart {
			_ = store.AbortMultipartUpload(context.Background(), storageKey, minioUploadID)
		}
		return InitUploadSessionResult{}, fmt.Errorf("create upload session: %w", err)
	}

	slog.InfoContext(ctx, "upload_session.init",
		"upload_id", sessionID,
		"library_id", cmd.LibraryID,
		"file_name", fileName,
		"file_size", cmd.FileSize,
		"mode", mode,
		"part_size", partSize,
	)

	return InitUploadSessionResult{
		UploadID:   sessionID,
		StorageKey: storageKey,
		Mode:       mode,
		PartSize:   partSize,
		TotalParts: totalParts,
		ExpiresAt:  expiresAt,
	}, nil
}

// SignParts 颁发分片预签名 URL，并隐式续约会话 lease。
func (u *UploadSessionUseCase) SignParts(ctx context.Context, cmd SignUploadPartsCommand) (SignUploadPartsResult, error) {
	session, err := u.loadSession(ctx, cmd.Actor, cmd.UploadID)
	if err != nil {
		return SignUploadPartsResult{}, err
	}
	if len(cmd.PartNumbers) == 0 {
		return SignUploadPartsResult{}, fmt.Errorf("%w: partNumbers must not be empty", ErrInvalidArgument)
	}

	store, err := u.registry.Get(session.StorageProvider)
	if err != nil {
		return SignUploadPartsResult{}, fmt.Errorf("storage provider %q: %w", session.StorageProvider, err)
	}

	expiry := cmd.Expiry
	if expiry <= 0 {
		expiry = presignedURLDefaultExpiry
	}
	urlExpiresAt := time.Now().UTC().Add(expiry)

	parts := make([]SignedUploadPart, 0, len(cmd.PartNumbers))
	for _, partNumber := range cmd.PartNumbers {
		if partNumber < 1 {
			return SignUploadPartsResult{}, fmt.Errorf("%w: part number must be >= 1", ErrInvalidArgument)
		}
		var (
			signedURL string
			signErr   error
		)
		if session.Mode == domainsession.ModeSingle {
			if partNumber != 1 {
				return SignUploadPartsResult{}, fmt.Errorf("%w: single mode only supports part 1", ErrInvalidArgument)
			}
			signedURL, signErr = store.PresignedPutObject(ctx, session.StorageKey, expiry)
		} else {
			signedURL, signErr = store.PresignedUploadPart(ctx, session.StorageKey, session.MinioUploadID, partNumber, expiry)
		}
		if signErr != nil {
			return SignUploadPartsResult{}, fmt.Errorf("presign part %d: %w", partNumber, signErr)
		}
		parts = append(parts, SignedUploadPart{
			PartNumber: partNumber,
			URL:        signedURL,
			ExpiresAt:  urlExpiresAt,
		})
	}

	leaseExpiresAt, err := u.renewLease(ctx, session.ID)
	if err != nil {
		return SignUploadPartsResult{}, err
	}

	return SignUploadPartsResult{
		Parts:     parts,
		ExpiresAt: leaseExpiresAt,
	}, nil
}

// ListParts 透传 MinIO ListParts，断点续传时由客户端比对已完成的 partNumber 跳过重传。
func (u *UploadSessionUseCase) ListParts(ctx context.Context, act actor.Actor, uploadID string) ([]UploadSessionPart, error) {
	session, err := u.loadSession(ctx, act, uploadID)
	if err != nil {
		return nil, err
	}
	if session.Mode == domainsession.ModeSingle {
		// single 模式没有 multipart 分片表，返回空数组让客户端按字节进度判断。
		_, _ = u.renewLease(ctx, session.ID)
		return []UploadSessionPart{}, nil
	}

	store, err := u.registry.Get(session.StorageProvider)
	if err != nil {
		return nil, fmt.Errorf("storage provider %q: %w", session.StorageProvider, err)
	}
	parts, err := store.ListParts(ctx, session.StorageKey, session.MinioUploadID)
	if err != nil {
		return nil, fmt.Errorf("list object parts: %w", err)
	}
	out := make([]UploadSessionPart, len(parts))
	for i, p := range parts {
		out[i] = UploadSessionPart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
			Size:       p.Size,
		}
	}
	_, _ = u.renewLease(ctx, session.ID)
	return out, nil
}

// Renew 显式续约 lease，仅用作客户端心跳兜底；不签发 URL。
func (u *UploadSessionUseCase) Renew(ctx context.Context, act actor.Actor, uploadID string) (time.Time, error) {
	session, err := u.loadSession(ctx, act, uploadID)
	if err != nil {
		return time.Time{}, err
	}
	return u.renewLease(ctx, session.ID)
}

// Complete 完成上传：提交 MinIO（multipart）或 HEAD 校验（single）→ 创建 node → 删 session 行 → audit。
func (u *UploadSessionUseCase) Complete(ctx context.Context, cmd CompleteUploadSessionCommand) (domainnode.Node, error) {
	session, err := u.loadSession(ctx, cmd.Actor, cmd.UploadID)
	if err != nil {
		return domainnode.Node{}, err
	}

	store, err := u.registry.Get(session.StorageProvider)
	if err != nil {
		return domainnode.Node{}, fmt.Errorf("storage provider %q: %w", session.StorageProvider, err)
	}

	if session.Mode == domainsession.ModeMultipart {
		if len(cmd.Parts) == 0 {
			return domainnode.Node{}, fmt.Errorf("%w: parts must not be empty for multipart upload", ErrInvalidArgument)
		}
		storageParts := make([]storage.MultipartUploadPart, len(cmd.Parts))
		for i, p := range cmd.Parts {
			storageParts[i] = storage.MultipartUploadPart{
				PartNumber: p.PartNumber,
				ETag:       p.ETag,
			}
		}
		if err := store.CompleteMultipartUpload(ctx, session.StorageKey, session.MinioUploadID, storageParts); err != nil {
			return domainnode.Node{}, fmt.Errorf("complete multipart: %w", err)
		}
	} else {
		// single 模式：HEAD 校验对象已写入且大小匹配。
		info, statErr := store.StatObject(ctx, session.StorageKey)
		if statErr != nil {
			return domainnode.Node{}, fmt.Errorf("%w: object not uploaded yet", ErrInvalidArgument)
		}
		if info.Size != session.FileSize {
			return domainnode.Node{}, fmt.Errorf("%w: stored size %d does not match declared size %d", ErrInvalidArgument, info.Size, session.FileSize)
		}
	}

	base := extractUploadBaseName(session.FileName)
	extWithDot := path.Ext(base)
	name := strings.TrimSuffix(base, extWithDot)
	if name == "" {
		name = base
	}
	ext := strings.TrimPrefix(extWithDot, ".")

	node, err := u.nodes.Create(ctx, CreateNodeCommand{
		Actor:           cmd.Actor,
		Name:            name,
		Type:            domainnode.TypeFile,
		ParentID:        session.ParentID,
		LibraryID:       session.LibraryID,
		Ext:             ext,
		MIMEType:        session.ContentType,
		FileSize:        session.FileSize,
		StorageKey:      session.StorageKey,
		StorageProvider: session.StorageProvider,
		StorageBucket:   store.Bucket(),
		ConflictPolicy:  cmd.ConflictPolicy,
	})
	if err != nil {
		// node 创建失败：回收 MinIO 对象，避免孤儿；session 行保留，等 janitor 收尾。
		_ = store.Delete(context.Background(), session.StorageKey)
		return domainnode.Node{}, err
	}

	if _, delErr := u.repo.Delete(ctx, session.ID); delErr != nil {
		slog.WarnContext(ctx, "upload_session.complete.delete_session_failed",
			"upload_id", session.ID,
			"error", delErr,
		)
	}

	_ = u.writeAudit(ctx, cmd.Actor, "upload.completed", true, map[string]any{
		"library_id":  session.LibraryID,
		"parent_id":   session.ParentID,
		"node_id":     node.ID,
		"name":        node.Name,
		"storage_key": session.StorageKey,
		"size":        session.FileSize,
		"mime_type":   session.ContentType,
		"mode":        session.Mode,
	})
	slog.InfoContext(ctx, "upload_session.complete",
		"upload_id", session.ID,
		"library_id", session.LibraryID,
		"node_id", node.ID,
		"size", session.FileSize,
	)
	return node, nil
}

// Abort 取消上传：回收 MinIO multipart（如适用）+ 删 session 行。
func (u *UploadSessionUseCase) Abort(ctx context.Context, act actor.Actor, uploadID string) error {
	session, err := u.loadSession(ctx, act, uploadID)
	if err != nil {
		return err
	}

	if session.Mode == domainsession.ModeMultipart && session.MinioUploadID != "" {
		store, storeErr := u.registry.Get(session.StorageProvider)
		if storeErr != nil {
			return fmt.Errorf("storage provider %q: %w", session.StorageProvider, storeErr)
		}
		if abortErr := store.AbortMultipartUpload(ctx, session.StorageKey, session.MinioUploadID); abortErr != nil {
			slog.WarnContext(ctx, "upload_session.abort.minio_failed",
				"upload_id", session.ID,
				"error", abortErr,
			)
		}
	}

	if _, delErr := u.repo.Delete(ctx, session.ID); delErr != nil {
		return fmt.Errorf("delete upload session: %w", delErr)
	}

	slog.InfoContext(ctx, "upload_session.abort",
		"upload_id", session.ID,
		"library_id", session.LibraryID,
	)
	return nil
}

// loadSession 拉取并校验 actor + 过期。actor 不匹配按 ErrNotFound 返回防枚举。
func (u *UploadSessionUseCase) loadSession(ctx context.Context, act actor.Actor, uploadID string) (domainsession.UploadSession, error) {
	id := strings.TrimSpace(uploadID)
	if id == "" {
		return domainsession.UploadSession{}, fmt.Errorf("%w: uploadId is required", ErrInvalidArgument)
	}
	session, err := u.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domainsession.UploadSession{}, fmt.Errorf("%w: upload session not found", ErrNotFound)
		}
		return domainsession.UploadSession{}, err
	}
	if session.ActorID != act.ID {
		return domainsession.UploadSession{}, fmt.Errorf("%w: upload session not found", ErrNotFound)
	}
	if !session.ExpiresAt.IsZero() && time.Now().UTC().After(session.ExpiresAt) {
		return domainsession.UploadSession{}, fmt.Errorf("%w: upload session lease expired", ErrExpired)
	}
	return session, nil
}

func (u *UploadSessionUseCase) renewLease(ctx context.Context, uploadID string) (time.Time, error) {
	expiresAt := time.Now().UTC().Add(u.sessionTTL())
	ok, err := u.repo.UpdateExpiresAt(ctx, uploadID, expiresAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("renew upload session: %w", err)
	}
	if !ok {
		return time.Time{}, fmt.Errorf("%w: upload session not found", ErrNotFound)
	}
	return expiresAt, nil
}

func (u *UploadSessionUseCase) sessionTTL() time.Duration {
	if u.cfg != nil && u.cfg.Upload.SessionTTL > 0 {
		return u.cfg.Upload.SessionTTL
	}
	return 24 * time.Hour
}

func (u *UploadSessionUseCase) resolveProvider(override string, fileSize int64, ext, contentType string) (storage.ObjectStorage, string, error) {
	if alias := strings.TrimSpace(override); alias != "" {
		store, err := u.registry.Get(alias)
		if err != nil {
			return nil, "", fmt.Errorf("%w: storage provider %q: %v", ErrInvalidArgument, alias, err)
		}
		return store, alias, nil
	}
	store, alias, err := u.registry.Resolve(fileSize, ext, contentType)
	if err != nil {
		return nil, "", err
	}
	return store, alias, nil
}

func (u *UploadSessionUseCase) authorize(ctx context.Context, principal actor.Actor, libraryID uint64, action authz.Action) error {
	if u.authorizer == nil {
		return nil
	}
	return u.authorizer.Authorize(ctx, principal, authz.Resource{
		Kind: "library",
		ID:   fmt.Sprintf("%d", libraryID),
	}, action)
}

func (u *UploadSessionUseCase) writeAudit(ctx context.Context, principal actor.Actor, action string, success bool, metadata map[string]any) error {
	if u.auditLog == nil {
		return nil
	}
	return u.auditLog.Write(ctx, audit.Event{
		Actor:      principal,
		Action:     action,
		Resource:   "upload_session",
		Success:    success,
		OccurredAt: time.Now().UTC(),
		Metadata:   metadata,
	})
}

// runJanitor 5 分钟扫一次 expires_at <= now 的会话：multipart 调 MinIO Abort，然后删行。
func (u *UploadSessionUseCase) runJanitor() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-u.stopCh:
			return
		case <-ticker.C:
			u.sweepExpired()
		}
	}
}

func (u *UploadSessionUseCase) sweepExpired() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	now := time.Now().UTC()
	sessions, err := u.repo.ListExpiredBefore(ctx, now, 100)
	if err != nil {
		slog.WarnContext(ctx, "upload_session.sweep.list_failed", "error", err)
		return
	}
	for _, s := range sessions {
		if s.Mode == domainsession.ModeMultipart && s.MinioUploadID != "" {
			store, storeErr := u.registry.Get(s.StorageProvider)
			if storeErr != nil {
				slog.WarnContext(ctx, "upload_session.sweep.provider_not_found",
					"upload_id", s.ID, "provider", s.StorageProvider, "error", storeErr)
			} else if abortErr := store.AbortMultipartUpload(ctx, s.StorageKey, s.MinioUploadID); abortErr != nil {
				slog.WarnContext(ctx, "upload_session.sweep.abort_failed",
					"upload_id", s.ID, "error", abortErr)
			}
		}
		if _, delErr := u.repo.Delete(ctx, s.ID); delErr != nil {
			slog.WarnContext(ctx, "upload_session.sweep.delete_failed",
				"upload_id", s.ID, "error", delErr)
		}
	}
}

func resolveDirectUploadContentType(declared, extWithDot string) string {
	contentType := strings.TrimSpace(declared)
	if contentType == "" || strings.EqualFold(contentType, defaultUploadContentType) {
		byExt := strings.TrimSpace(mime.TypeByExtension(extWithDot))
		if byExt != "" {
			contentType = byExt
		}
	}
	if contentType == "" {
		contentType = defaultUploadContentType
	}
	return contentType
}

// pickUploadMode 决定单/多分片模式与分片大小、总分片数。
// single：≤ 阈值，1 part；multipart：partSize 取 cfg.ChunkSize 与默认 partSize 的较大值，向上取整算 totalParts。
func pickUploadMode(fileSize, cfgChunkSize int64) (mode string, partSize int64, totalParts int) {
	if fileSize <= directUploadSingleThresholdBytes {
		return domainsession.ModeSingle, fileSize, 1
	}
	partSize = cfgChunkSize
	if partSize < directUploadDefaultPartSizeBytes {
		partSize = directUploadDefaultPartSizeBytes
	}
	totalParts = int((fileSize + partSize - 1) / partSize)
	return domainsession.ModeMultipart, partSize, totalParts
}
