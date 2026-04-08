package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	domainuser "omniflow-go/internal/domain/user"
	"omniflow-go/internal/repository"
	"omniflow-go/internal/storage"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	userAvatarExtKey       = "avatarKey"
	userAvatarLegacyExtKey = "avatar"
	userAvatarURLExpiry    = 60 * time.Minute
)

var allowedAvatarExtensions = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".webp": {},
	".gif":  {},
	".bmp":  {},
	".svg":  {},
}

type RegisterUserCommand struct {
	Actor    actor.Actor
	Username string
	Password string
	Phone    string
	Email    string
}

type UpdateUserCommand struct {
	Actor    actor.Actor
	ID       uint64
	Username *string
	Password *string
	Nickname *string
	Phone    *string
	Email    *string
	Ext      *string
}

type UpdateCurrentPasswordCommand struct {
	Actor       actor.Actor
	OldPassword string
	NewPassword string
}

type UploadCurrentUserAvatarCommand struct {
	Actor       actor.Actor
	FileName    string
	FileSize    int64
	ContentType string
	Content     io.Reader
}

type UserUseCase struct {
	users    *repository.UserRepository
	redis    *redis.Client
	storage  storage.ObjectStorage
	auditLog audit.Sink
}

func NewUserUseCase(
	users *repository.UserRepository,
	redis *redis.Client,
	storage storage.ObjectStorage,
	auditLog ...audit.Sink,
) *UserUseCase {
	setSharedRedisClient(redis)

	var sink audit.Sink
	if len(auditLog) > 0 {
		sink = auditLog[0]
	}

	return &UserUseCase{
		users:    users,
		redis:    redis,
		storage:  storage,
		auditLog: sink,
	}
}

func (u *UserUseCase) GetByUsername(ctx context.Context, username string) (domainuser.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return domainuser.User{}, fmt.Errorf("%w: username is required", ErrInvalidArgument)
	}

	db, err := dbFromRepository(u.users)
	if err != nil {
		return domainuser.User{}, err
	}

	var user userRecord
	if err := db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainuser.User{}, ErrNotFound
		}
		return domainuser.User{}, err
	}
	return u.enrichUser(ctx, user.toDomain()), nil
}

func (u *UserUseCase) GetCurrent(ctx context.Context, principal actor.Actor) (domainuser.User, error) {
	userID, err := actorIDToUint64(principal)
	if err != nil {
		return domainuser.User{}, err
	}

	record, err := u.getUserRecordByID(ctx, userID)
	if err != nil {
		return domainuser.User{}, err
	}
	return u.enrichUser(ctx, record.toDomain()), nil
}

func (u *UserUseCase) Exists(ctx context.Context, username string) (bool, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return false, fmt.Errorf("%w: username is required", ErrInvalidArgument)
	}

	db, err := dbFromRepository(u.users)
	if err != nil {
		return false, err
	}

	var count int64
	if err := db.WithContext(ctx).
		Model(&userRecord{}).
		Where("username = ?", username).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (u *UserUseCase) Register(ctx context.Context, cmd RegisterUserCommand) (domainuser.User, error) {
	username := strings.TrimSpace(cmd.Username)
	password := strings.TrimSpace(cmd.Password)
	if username == "" || password == "" {
		return domainuser.User{}, fmt.Errorf("%w: username and password are required", ErrInvalidArgument)
	}

	exists, err := u.Exists(ctx, username)
	if err != nil {
		return domainuser.User{}, err
	}
	if exists {
		return domainuser.User{}, ErrConflict
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return domainuser.User{}, err
	}

	record := userRecord{
		Username:     username,
		Nickname:     username,
		PasswordHash: string(hashed),
		Phone:        strings.TrimSpace(cmd.Phone),
		Email:        strings.TrimSpace(cmd.Email),
		Status:       userStatusActive,
	}

	db, err := dbFromRepository(u.users)
	if err != nil {
		return domainuser.User{}, err
	}
	if err := db.WithContext(ctx).Create(&record).Error; err != nil {
		return domainuser.User{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "user.register", true, map[string]any{
		"user_id":   record.ID,
		"username":  record.Username,
		"has_phone": record.Phone != "",
		"has_email": record.Email != "",
	})
	return u.enrichUser(ctx, record.toDomain()), nil
}

func (u *UserUseCase) Update(ctx context.Context, cmd UpdateUserCommand) (domainuser.User, error) {
	db, err := dbFromRepository(u.users)
	if err != nil {
		return domainuser.User{}, err
	}

	targetID, err := u.resolveTargetUserID(cmd)
	if err != nil {
		return domainuser.User{}, err
	}
	if err := u.ensureSelfPermission(cmd.Actor, targetID); err != nil {
		return domainuser.User{}, err
	}

	var existing userRecord
	if err := db.WithContext(ctx).First(&existing, "id = ?", targetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainuser.User{}, ErrNotFound
		}
		return domainuser.User{}, err
	}

	updates := map[string]any{}
	if cmd.Username != nil {
		updates["username"] = strings.TrimSpace(*cmd.Username)
	}
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
	if cmd.Password != nil {
		pwd := strings.TrimSpace(*cmd.Password)
		if pwd == "" {
			return domainuser.User{}, fmt.Errorf("%w: password cannot be empty", ErrInvalidArgument)
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(pwd), 10)
		if err != nil {
			return domainuser.User{}, err
		}
		updates["password_hash"] = string(hashed)
	}

	if len(updates) == 0 {
		return u.enrichUser(ctx, existing.toDomain()), nil
	}
	updates["updated_at"] = time.Now().UTC()

	if err := db.WithContext(ctx).
		Model(&userRecord{}).
		Where("id = ?", targetID).
		Updates(updates).Error; err != nil {
		return domainuser.User{}, err
	}

	var updated userRecord
	if err := db.WithContext(ctx).First(&updated, "id = ?", targetID).Error; err != nil {
		return domainuser.User{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "user.update", true, map[string]any{
		"user_id": targetID,
		"fields":  len(updates) - 1, // exclude updated_at
	})
	return u.enrichUser(ctx, updated.toDomain()), nil
}

func (u *UserUseCase) UpdateCurrentPassword(ctx context.Context, cmd UpdateCurrentPasswordCommand) error {
	userID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return err
	}

	oldPassword := strings.TrimSpace(cmd.OldPassword)
	newPassword := strings.TrimSpace(cmd.NewPassword)
	if oldPassword == "" {
		return fmt.Errorf("%w: oldPassword is required", ErrInvalidArgument)
	}
	if newPassword == "" {
		return fmt.Errorf("%w: newPassword is required", ErrInvalidArgument)
	}

	db, err := dbFromRepository(u.users)
	if err != nil {
		return err
	}

	var existing userRecord
	if err := db.WithContext(ctx).First(&existing, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(existing.PasswordHash), []byte(oldPassword)); err != nil {
		return fmt.Errorf("%w: old password mismatch", ErrInvalidArgument)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(existing.PasswordHash), []byte(newPassword)); err == nil {
		return fmt.Errorf("%w: new password cannot be same as old password", ErrInvalidArgument)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), 10)
	if err != nil {
		return err
	}

	if err := db.WithContext(ctx).
		Model(&userRecord{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"password_hash": string(hashed),
			"updated_at":    time.Now().UTC(),
		}).Error; err != nil {
		return err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "user.password.update", true, map[string]any{
		"user_id": userID,
	})
	return nil
}

func (u *UserUseCase) UploadCurrentAvatar(ctx context.Context, cmd UploadCurrentUserAvatarCommand) (domainuser.User, error) {
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

	extWithDot := strings.ToLower(filepath.Ext(strings.TrimSpace(cmd.FileName)))
	if _, ok := allowedAvatarExtensions[extWithDot]; !ok {
		return domainuser.User{}, fmt.Errorf("%w: avatar only supports jpg/png/webp/gif/bmp/svg", ErrInvalidArgument)
	}

	contentType := strings.TrimSpace(cmd.ContentType)
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = mime.TypeByExtension(extWithDot)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	db, err := dbFromRepository(u.users)
	if err != nil {
		return domainuser.User{}, err
	}

	var existing userRecord
	if err := db.WithContext(ctx).First(&existing, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainuser.User{}, ErrNotFound
		}
		return domainuser.User{}, err
	}

	extJSON := parseUserExtJSON(existing.ExtJSON)
	oldAvatarKey := strings.TrimSpace(stringValue(extJSON[userAvatarExtKey]))
	newAvatarKey := fmt.Sprintf("users/%d/avatar/%s%s", userID, uuid.NewString(), extWithDot)

	if err := u.storage.Upload(ctx, newAvatarKey, cmd.Content, cmd.FileSize, contentType); err != nil {
		return domainuser.User{}, err
	}

	extJSON[userAvatarExtKey] = newAvatarKey
	delete(extJSON, userAvatarLegacyExtKey)
	extRaw, err := json.Marshal(extJSON)
	if err != nil {
		_ = u.storage.Delete(ctx, newAvatarKey)
		return domainuser.User{}, err
	}

	if err := db.WithContext(ctx).
		Model(&userRecord{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"ext":        string(extRaw),
			"updated_at": time.Now().UTC(),
		}).Error; err != nil {
		_ = u.storage.Delete(ctx, newAvatarKey)
		return domainuser.User{}, err
	}

	if oldAvatarKey != "" && oldAvatarKey != newAvatarKey {
		_ = u.storage.Delete(ctx, oldAvatarKey)
	}

	refreshed, err := u.getUserRecordByID(ctx, userID)
	if err != nil {
		return domainuser.User{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "user.avatar.upload", true, map[string]any{
		"user_id":    userID,
		"avatar_key": newAvatarKey,
	})
	return u.enrichUser(ctx, refreshed.toDomain()), nil
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

func (u *UserUseCase) getUserRecordByID(ctx context.Context, userID uint64) (userRecord, error) {
	db, err := dbFromRepository(u.users)
	if err != nil {
		return userRecord{}, err
	}

	var user userRecord
	if err := db.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return userRecord{}, ErrNotFound
		}
		return userRecord{}, err
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
