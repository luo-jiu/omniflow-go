package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	domainuser "omniflow-go/internal/domain/user"
	"omniflow-go/internal/repository"
	"omniflow-go/internal/storage"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"golang.org/x/crypto/bcrypt"
)

const (
	userAvatarExtKey       = "avatarKey"
	userAvatarLegacyExtKey = "avatar"
	userAvatarURLExpiry    = 24 * time.Hour
)

var allowedAvatarExtensions = lo.SliceToMap([]string{
	".jpg",
	".jpeg",
	".png",
	".gif",
	".bmp",
	".ico",
	".tif",
	".tiff",
	".webp",
	".avif",
	".heic",
	".heif",
}, func(ext string) (string, struct{}) {
	return ext, struct{}{}
})

type RegisterUserCommand struct {
	Actor    actor.Actor
	Username string
	Nickname string
	Password string
	Phone    string
	Email    string
	Ext      string
}

type UpdateUserCommand struct {
	Actor    actor.Actor
	ID       uint64
	Nickname *string
	Phone    *string
	Email    *string
	Ext      *string
	DryRun   bool
}

type UpdateCurrentPasswordCommand struct {
	Actor       actor.Actor
	OldPassword string
	NewPassword string
	DryRun      bool
}

type UploadCurrentUserAvatarCommand struct {
	Actor       actor.Actor
	FileName    string
	FileSize    int64
	ContentType string
	Content     io.Reader
	DryRun      bool
}

type UserUseCase struct {
	users    *repository.UserRepository
	storage  storage.ObjectStorage
	tx       repository.Transactor
	auditLog audit.Sink
}

func NewUserUseCase(
	users *repository.UserRepository,
	storage storage.ObjectStorage,
	tx repository.Transactor,
	auditLog ...audit.Sink,
) *UserUseCase {
	var sink audit.Sink
	if len(auditLog) > 0 {
		sink = auditLog[0]
	}

	return &UserUseCase{
		users:    users,
		storage:  storage,
		tx:       tx,
		auditLog: sink,
	}
}

func (u *UserUseCase) GetByUsername(ctx context.Context, username string) (domainuser.User, error) {
	if err := u.ensureUsersConfigured(); err != nil {
		return domainuser.User{}, err
	}

	username = strings.TrimSpace(username)
	if username == "" {
		slog.WarnContext(ctx, "user.by_username.invalid_argument", "reason", "username_missing")
		return domainuser.User{}, fmt.Errorf("%w: username is required", ErrInvalidArgument)
	}

	user, err := u.users.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			slog.DebugContext(ctx, "user.by_username.not_found", "username", username)
			return domainuser.User{}, ErrNotFound
		}
		return domainuser.User{}, err
	}
	slog.DebugContext(ctx, "user.by_username.fetched",
		"username", username,
		"user_id", user.ID,
	)
	return u.enrichUser(ctx, user), nil
}

func (u *UserUseCase) GetCurrent(ctx context.Context, principal actor.Actor) (domainuser.User, error) {
	if err := u.ensureUsersConfigured(); err != nil {
		return domainuser.User{}, err
	}

	userID, err := actorIDToUint64(principal)
	if err != nil {
		return domainuser.User{}, err
	}

	record, err := u.getUserByID(ctx, userID)
	if err != nil {
		return domainuser.User{}, err
	}
	slog.DebugContext(ctx, "user.profile.fetched", "user_id", userID)
	return u.enrichUser(ctx, record), nil
}

func (u *UserUseCase) Exists(ctx context.Context, username string) (bool, error) {
	if err := u.ensureUsersConfigured(); err != nil {
		return false, err
	}

	username = strings.TrimSpace(username)
	if username == "" {
		slog.WarnContext(ctx, "user.exists.invalid_argument", "reason", "username_missing")
		return false, fmt.Errorf("%w: username is required", ErrInvalidArgument)
	}

	exists, err := u.users.ExistsByUsername(ctx, username)
	if err != nil {
		return false, err
	}
	slog.DebugContext(ctx, "user.exists.checked",
		"username", username,
		"exists", exists,
	)
	return exists, nil
}

// HasUsername 与 Java 语义保持一致：返回“用户名是否可用（未被占用）”。
func (u *UserUseCase) HasUsername(ctx context.Context, username string) (bool, error) {
	exists, err := u.Exists(ctx, username)
	if err != nil {
		return false, err
	}
	available := !exists
	slog.DebugContext(ctx, "user.username_availability.checked",
		"username", strings.TrimSpace(username),
		"available", available,
	)
	return available, nil
}

func (u *UserUseCase) Register(ctx context.Context, cmd RegisterUserCommand) (domainuser.User, error) {
	if err := u.ensureUsersConfigured(); err != nil {
		return domainuser.User{}, err
	}

	username := strings.TrimSpace(cmd.Username)
	password := strings.TrimSpace(cmd.Password)
	if username == "" || password == "" {
		slog.WarnContext(ctx, "user.register.invalid_argument",
			"username", username,
			"has_password", password != "",
		)
		return domainuser.User{}, fmt.Errorf("%w: username and password are required", ErrInvalidArgument)
	}

	exists, err := u.Exists(ctx, username)
	if err != nil {
		return domainuser.User{}, err
	}
	if exists {
		slog.InfoContext(ctx, "user.register.blocked",
			"username", username,
			"reason", "username_conflict",
		)
		return domainuser.User{}, ErrConflict
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return domainuser.User{}, err
	}

	nicknameInput := strings.TrimSpace(cmd.Nickname)
	nickname := username
	if nicknameInput != "" {
		nickname = nicknameInput
	}

	created, err := u.users.Create(ctx, repository.CreateUserInput{
		Username:     username,
		Nickname:     nickname,
		PasswordHash: string(hashed),
		Phone:        strings.TrimSpace(cmd.Phone),
		Email:        strings.TrimSpace(cmd.Email),
		Ext:          strings.TrimSpace(cmd.Ext),
	})
	if err != nil {
		return domainuser.User{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "user.register", true, map[string]any{
		"user_id":   created.ID,
		"username":  created.Username,
		"has_phone": created.Phone != "",
		"has_email": created.Email != "",
	})
	slog.InfoContext(ctx, "user.registered",
		"user_id", created.ID,
		"username", created.Username,
	)
	return u.enrichUser(ctx, created), nil
}

func (u *UserUseCase) Update(ctx context.Context, cmd UpdateUserCommand) (domainuser.User, error) {
	if err := u.ensureUsersConfigured(); err != nil {
		return domainuser.User{}, err
	}

	targetID, err := u.resolveTargetUserID(cmd)
	if err != nil {
		return domainuser.User{}, err
	}
	if err := u.ensureSelfPermission(cmd.Actor, targetID); err != nil {
		return domainuser.User{}, err
	}

	existing, err := u.users.FindByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domainuser.User{}, ErrNotFound
		}
		return domainuser.User{}, err
	}

	updates := map[string]any{}
	if cmd.Nickname != nil {
		nickname := strings.TrimSpace(*cmd.Nickname)
		if nickname == "" {
			return domainuser.User{}, fmt.Errorf("%w: nickname cannot be empty", ErrInvalidArgument)
		}
		updates["nickname"] = nickname
	}
	if cmd.Phone != nil {
		updates["phone"] = strings.TrimSpace(*cmd.Phone)
	}
	if cmd.Email != nil {
		updates["email"] = strings.TrimSpace(*cmd.Email)
	}
	if cmd.Ext != nil {
		updates["ext"] = strings.TrimSpace(*cmd.Ext)
	}
	if len(updates) == 0 {
		return u.enrichUser(ctx, existing), nil
	}
	updates["updated_at"] = time.Now().UTC()

	var updated domainuser.User
	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		ok, err := u.users.UpdateByID(txCtx, targetID, updates)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}

		row, err := u.users.FindByID(txCtx, targetID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}
		updated = row
		return nil
	}); err != nil {
		return domainuser.User{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "user.update", true, map[string]any{
		"user_id": targetID,
		"fields":  len(updates) - 1, // exclude updated_at
		"mode":    resolveMutationMode(cmd.DryRun),
		"dry_run": cmd.DryRun,
	})
	slog.InfoContext(ctx, "user.profile.updated",
		"user_id", targetID,
		"fields", len(updates)-1,
		"dry_run", cmd.DryRun,
	)
	return u.enrichUser(ctx, updated), nil
}

func (u *UserUseCase) UpdateCurrentPassword(ctx context.Context, cmd UpdateCurrentPasswordCommand) error {
	if err := u.ensureUsersConfigured(); err != nil {
		return err
	}

	userID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return err
	}

	oldPassword := strings.TrimSpace(cmd.OldPassword)
	newPassword := strings.TrimSpace(cmd.NewPassword)
	if oldPassword == "" {
		slog.WarnContext(ctx, "user.password.update.invalid_argument", "reason", "old_password_missing", "dry_run", cmd.DryRun)
		return fmt.Errorf("%w: oldPassword is required", ErrInvalidArgument)
	}
	if newPassword == "" {
		slog.WarnContext(ctx, "user.password.update.invalid_argument", "reason", "new_password_missing", "dry_run", cmd.DryRun)
		return fmt.Errorf("%w: newPassword is required", ErrInvalidArgument)
	}

	existing, err := u.users.FindAuthByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(existing.PasswordHash), []byte(oldPassword)); err != nil {
		slog.InfoContext(ctx, "user.password.update.blocked", "user_id", userID, "reason", "old_password_mismatch", "dry_run", cmd.DryRun)
		return fmt.Errorf("%w: old password mismatch", ErrInvalidArgument)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(existing.PasswordHash), []byte(newPassword)); err == nil {
		slog.InfoContext(ctx, "user.password.update.blocked", "user_id", userID, "reason", "new_password_same_as_old", "dry_run", cmd.DryRun)
		return fmt.Errorf("%w: new password cannot be same as old password", ErrInvalidArgument)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), 10)
	if err != nil {
		return err
	}

	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		ok, err := u.users.UpdateByID(txCtx, userID, map[string]any{
			"password_hash": string(hashed),
			"updated_at":    time.Now().UTC(),
		})
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		return nil
	}); err != nil {
		return err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "user.password.update", true, map[string]any{
		"user_id": userID,
		"mode":    resolveMutationMode(cmd.DryRun),
		"dry_run": cmd.DryRun,
	})
	slog.InfoContext(ctx, "user.password.updated", "user_id", userID, "dry_run", cmd.DryRun)
	return nil
}

func (u *UserUseCase) UploadCurrentAvatar(ctx context.Context, cmd UploadCurrentUserAvatarCommand) (domainuser.User, error) {
	if err := u.ensureUsersConfigured(); err != nil {
		return domainuser.User{}, err
	}
	if u.storage == nil {
		return domainuser.User{}, fmt.Errorf("%w: object storage not configured", ErrInvalidArgument)
	}
	if cmd.Content == nil {
		return domainuser.User{}, fmt.Errorf("%w: avatar file is required", ErrInvalidArgument)
	}
	if cmd.FileSize <= 0 {
		return domainuser.User{}, fmt.Errorf("%w: avatar file is empty", ErrInvalidArgument)
	}

	userID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return domainuser.User{}, err
	}

	uploadContent, resolvedExt, contentType, err := normalizeAvatarUpload(cmd.Content, cmd.FileName, cmd.ContentType)
	if err != nil {
		return domainuser.User{}, err
	}

	existing, err := u.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domainuser.User{}, ErrNotFound
		}
		return domainuser.User{}, err
	}

	extJSON := parseUserExtJSON(existing.Ext)
	oldAvatarKey := strings.TrimSpace(stringValue(extJSON[userAvatarExtKey]))
	newAvatarKey := buildAvatarStorageKey(userID, resolvedExt)

	if cmd.DryRun {
		extJSON[userAvatarExtKey] = newAvatarKey
		delete(extJSON, userAvatarLegacyExtKey)
		extRaw, err := json.Marshal(extJSON)
		if err != nil {
			return domainuser.User{}, err
		}

		// dry-run 只返回“将会写入的 ext”，头像 URL 仍保持当前可用值，避免误导调用方访问不存在对象。
		simulated := u.enrichUser(ctx, existing)
		simulated.Ext = string(extRaw)
		_ = u.writeAudit(ctx, cmd.Actor, "user.avatar.upload", true, map[string]any{
			"user_id":    userID,
			"avatar_key": newAvatarKey,
			"mode":       resolveMutationMode(true),
			"dry_run":    true,
		})
		slog.InfoContext(ctx, "user.avatar.upload.dry_run",
			"user_id", userID,
			"avatar_key", newAvatarKey,
			"dry_run", true,
		)
		return simulated, nil
	}

	if err := u.storage.Upload(ctx, newAvatarKey, uploadContent, cmd.FileSize, contentType); err != nil {
		return domainuser.User{}, err
	}

	extJSON[userAvatarExtKey] = newAvatarKey
	delete(extJSON, userAvatarLegacyExtKey)
	extRaw, err := json.Marshal(extJSON)
	if err != nil {
		_ = u.storage.Delete(ctx, newAvatarKey)
		return domainuser.User{}, err
	}

	ok, err := u.users.UpdateByID(ctx, userID, map[string]any{
		"ext":        string(extRaw),
		"updated_at": time.Now().UTC(),
	})
	if err != nil {
		_ = u.storage.Delete(ctx, newAvatarKey)
		return domainuser.User{}, err
	}
	if !ok {
		_ = u.storage.Delete(ctx, newAvatarKey)
		return domainuser.User{}, ErrNotFound
	}

	if oldAvatarKey != "" && oldAvatarKey != newAvatarKey {
		_ = u.storage.Delete(ctx, oldAvatarKey)
	}

	refreshed, err := u.getUserByID(ctx, userID)
	if err != nil {
		return domainuser.User{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "user.avatar.upload", true, map[string]any{
		"user_id":    userID,
		"avatar_key": newAvatarKey,
		"mode":       resolveMutationMode(false),
		"dry_run":    false,
	})
	slog.InfoContext(ctx, "user.avatar.uploaded",
		"user_id", userID,
		"avatar_key", newAvatarKey,
		"dry_run", false,
	)
	return u.enrichUser(ctx, refreshed), nil
}

func (u *UserUseCase) resolveTargetUserID(cmd UpdateUserCommand) (uint64, error) {
	targetID := cmd.ID
	if targetID > 0 {
		return targetID, nil
	}
	if cmd.Actor.IsZero() {
		return 0, fmt.Errorf("%w: user id is required", ErrInvalidArgument)
	}
	return actorIDToUint64(cmd.Actor)
}

func (u *UserUseCase) ensureSelfPermission(principal actor.Actor, targetID uint64) error {
	if principal.IsZero() {
		return nil
	}
	actorID, err := actorIDToUint64(principal)
	if err != nil {
		return err
	}
	if actorID != targetID {
		return ErrForbidden
	}
	return nil
}

func (u *UserUseCase) getUserByID(ctx context.Context, userID uint64) (domainuser.User, error) {
	user, err := u.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domainuser.User{}, ErrNotFound
		}
		return domainuser.User{}, err
	}
	return user, nil
}

func (u *UserUseCase) enrichUser(ctx context.Context, in domainuser.User) domainuser.User {
	in.Avatar = u.resolveAvatarURL(ctx, in.Ext)
	if strings.TrimSpace(in.Nickname) == "" {
		in.Nickname = in.Username
	}
	return in
}

func (u *UserUseCase) resolveAvatarURL(ctx context.Context, extRaw string) string {
	extJSON := parseUserExtJSON(extRaw)
	avatarKey := strings.TrimSpace(stringValue(extJSON[userAvatarExtKey]))
	if avatarKey != "" && u.storage != nil {
		url, err := u.storage.GetPresignedURL(ctx, avatarKey, userAvatarURLExpiry)
		if err == nil {
			return url
		}
	}
	return strings.TrimSpace(stringValue(extJSON[userAvatarLegacyExtKey]))
}

func resolveAvatarUploadExtByMIME(contentType string) string {
	contentType = normalizeMIMEType(contentType)
	switch contentType {
	case "image/jpg", "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/bmp":
		return ".bmp"
	case "image/vnd.microsoft.icon", "image/x-icon":
		return ".ico"
	case "image/tiff":
		return ".tiff"
	case "image/webp":
		return ".webp"
	case "image/avif":
		return ".avif"
	case "image/heic":
		return ".heic"
	case "image/heif":
		return ".heif"
	default:
		if !strings.HasPrefix(contentType, "image/") {
			return ""
		}
		subType := strings.TrimPrefix(contentType, "image/")
		if idx := strings.Index(subType, "+"); idx >= 0 {
			subType = subType[:idx]
		}
		subType = strings.TrimPrefix(subType, "x-")
		if subType == "" {
			return ""
		}

		ext := "." + subType
		if !isAllowedAvatarExt(ext) {
			return ""
		}
		return ext
	}
}

func normalizeAvatarUpload(content io.Reader, fileName, declaredContentType string) (io.Reader, string, string, error) {
	extWithDot := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	extAllowed := isAllowedAvatarExt(extWithDot)

	head := make([]byte, 512)
	readCount, readErr := io.ReadFull(content, head)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return nil, "", "", readErr
	}
	head = head[:readCount]

	detectedMIME := ""
	if len(head) > 0 {
		detectedMIME = normalizeMIMEType(http.DetectContentType(head))
	}
	declaredMIME := normalizeMIMEType(declaredContentType)
	mimeAllowed := lo.SomeBy([]string{detectedMIME, declaredMIME}, func(contentType string) bool {
		return isAllowedAvatarMIME(contentType)
	})
	if !extAllowed && !mimeAllowed {
		return nil, "", "", fmt.Errorf("%w: avatar only supports image files", ErrInvalidArgument)
	}

	resolvedExt, ok := lo.Find([]string{
		extWithDot,
		resolveAvatarUploadExtByMIME(detectedMIME),
		resolveAvatarUploadExtByMIME(declaredMIME),
	}, func(ext string) bool {
		return isAllowedAvatarExt(ext)
	})
	if !ok {
		return nil, "", "", fmt.Errorf("%w: avatar image format is not supported", ErrInvalidArgument)
	}

	resolvedContentType, ok := lo.Find([]string{
		detectedMIME,
		declaredMIME,
		resolveAvatarContentType(resolvedExt),
	}, func(contentType string) bool {
		return isAllowedAvatarMIME(contentType)
	})
	if !ok {
		resolvedContentType = "application/octet-stream"
	}

	rewindContent := io.MultiReader(bytes.NewReader(head), content)
	return rewindContent, resolvedExt, resolvedContentType, nil
}

func normalizeMIMEType(contentType string) string {
	normalized := strings.ToLower(strings.TrimSpace(contentType))
	if normalized == "" {
		return ""
	}
	if idx := strings.Index(normalized, ";"); idx >= 0 {
		normalized = strings.TrimSpace(normalized[:idx])
	}
	if normalized == "application/octet-stream" {
		return ""
	}
	return normalized
}

func isAllowedAvatarExt(extWithDot string) bool {
	if extWithDot == "" {
		return false
	}
	_, ok := allowedAvatarExtensions[strings.ToLower(strings.TrimSpace(extWithDot))]
	return ok
}

func isAllowedAvatarMIME(contentType string) bool {
	contentType = normalizeMIMEType(contentType)
	if contentType == "" {
		return false
	}
	if contentType == "image/svg+xml" {
		return false
	}
	return strings.HasPrefix(contentType, "image/")
}

func resolveAvatarContentType(extWithDot string) string {
	switch strings.ToLower(strings.TrimSpace(extWithDot)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".webp":
		return "image/webp"
	case ".avif":
		return "image/avif"
	case ".heic":
		return "image/heic"
	case ".heif":
		return "image/heif"
	default:
		return mime.TypeByExtension(extWithDot)
	}
}

func buildAvatarStorageKey(userID uint64, extWithDot string) string {
	normalizedExt := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extWithDot)), ".")
	if normalizedExt == "" {
		normalizedExt = "bin"
	}

	datePath := time.Now().UTC().Format("2006/01")
	id := strings.ReplaceAll(uuid.NewString(), "-", "")
	return fmt.Sprintf("user/%d/avatar/%s/%s.%s", userID, datePath, id, normalizedExt)
}

func parseUserExtJSON(raw string) map[string]any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return map[string]any{}
	}
	if parsed == nil {
		return map[string]any{}
	}
	return parsed
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func (u *UserUseCase) withinMutationTx(ctx context.Context, dryRun bool, fn func(ctx context.Context) error) error {
	if !dryRun {
		if u.tx == nil {
			return fn(ctx)
		}
		return u.tx.WithinTx(ctx, fn)
	}
	if u.tx == nil {
		return fmt.Errorf("%w: dry-run requires transaction manager", ErrInvalidArgument)
	}

	err := u.tx.WithinTx(ctx, func(txCtx context.Context) error {
		if err := fn(txCtx); err != nil {
			return err
		}
		return errUsecaseDryRunRollback
	})
	if err != nil && !errors.Is(err, errUsecaseDryRunRollback) {
		return err
	}
	return nil
}

func (u *UserUseCase) ensureUsersConfigured() error {
	if u.users != nil {
		return nil
	}
	return fmt.Errorf("%w: user repository not configured", ErrInvalidArgument)
}

func (u *UserUseCase) writeAudit(ctx context.Context, principal actor.Actor, action string, success bool, metadata map[string]any) error {
	if u.auditLog == nil {
		return nil
	}
	return u.auditLog.Write(ctx, audit.Event{
		Actor:      principal,
		Action:     action,
		Resource:   "user",
		Success:    success,
		OccurredAt: time.Now().UTC(),
		Metadata:   metadata,
	})
}
