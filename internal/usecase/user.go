package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	domainuser "omniflow-go/internal/domain/user"
	"omniflow-go/internal/repository"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

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
	Phone    *string
	Email    *string
}

type UserUseCase struct {
	users    *repository.UserRepository
	redis    *redis.Client
	auditLog audit.Sink
}

func NewUserUseCase(users *repository.UserRepository, redis *redis.Client, auditLog ...audit.Sink) *UserUseCase {
	setSharedRedisClient(redis)

	var sink audit.Sink
	if len(auditLog) > 0 {
		sink = auditLog[0]
	}

	return &UserUseCase{
		users:    users,
		redis:    redis,
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
	return user.toDomain(), nil
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
	return record.toDomain(), nil
}

func (u *UserUseCase) Update(ctx context.Context, cmd UpdateUserCommand) (domainuser.User, error) {
	db, err := dbFromRepository(u.users)
	if err != nil {
		return domainuser.User{}, err
	}

	targetID := cmd.ID
	if targetID == 0 && !cmd.Actor.IsZero() {
		targetID, err = actorIDToUint64(cmd.Actor)
		if err != nil {
			return domainuser.User{}, err
		}
	}
	if targetID == 0 {
		return domainuser.User{}, fmt.Errorf("%w: user id is required", ErrInvalidArgument)
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
	if cmd.Phone != nil {
		updates["phone"] = strings.TrimSpace(*cmd.Phone)
	}
	if cmd.Email != nil {
		updates["email"] = strings.TrimSpace(*cmd.Email)
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
		return existing.toDomain(), nil
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
	return updated.toDomain(), nil
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
