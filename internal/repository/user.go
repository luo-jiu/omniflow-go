package repository

import (
	"context"
	"errors"
	"strings"

	domainuser "omniflow-go/internal/domain/user"

	"gorm.io/gorm"
)

const (
	userStatusActive   = 1
	userStatusDisabled = 2
	userStatusPending  = 3
)

type UserRepository struct {
	db *gorm.DB
}

type UserAuth struct {
	User         domainuser.User
	PasswordHash string
}

type CreateUserInput struct {
	Username     string
	Nickname     string
	PasswordHash string
	Phone        string
	Email        string
	Ext          string
}

type userModel struct {
	ID           uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	Username     string         `gorm:"column:username"`
	Nickname     string         `gorm:"column:nickname"`
	PasswordHash string         `gorm:"column:password_hash"`
	Phone        string         `gorm:"column:phone"`
	Email        string         `gorm:"column:email"`
	ExtJSON      string         `gorm:"column:ext"`
	Status       int            `gorm:"column:status"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (userModel) TableName() string {
	return "users"
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) WithTx(tx *gorm.DB) *UserRepository {
	if tx == nil {
		return r
	}
	return &UserRepository{db: tx}
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (domainuser.User, error) {
	var model userModel
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&model).Error; err != nil {
		return domainuser.User{}, mapDBError(err)
	}
	return model.toDomain(), nil
}

func (r *UserRepository) FindByID(ctx context.Context, userID uint64) (domainuser.User, error) {
	var model userModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", userID).Error; err != nil {
		return domainuser.User{}, mapDBError(err)
	}
	return model.toDomain(), nil
}

func (r *UserRepository) FindAuthByID(ctx context.Context, userID uint64) (UserAuth, error) {
	var model userModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", userID).Error; err != nil {
		return UserAuth{}, mapDBError(err)
	}
	return UserAuth{
		User:         model.toDomain(),
		PasswordHash: model.PasswordHash,
	}, nil
}

func (r *UserRepository) FindActiveByUsername(ctx context.Context, username string) (UserAuth, error) {
	var model userModel
	if err := r.db.WithContext(ctx).
		Where("username = ? AND status = ?", username, userStatusActive).
		First(&model).Error; err != nil {
		return UserAuth{}, mapDBError(err)
	}
	return UserAuth{
		User:         model.toDomain(),
		PasswordHash: model.PasswordHash,
	}, nil
}

func (r *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&userModel{}).
		Where("username = ?", username).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepository) Create(ctx context.Context, input CreateUserInput) (domainuser.User, error) {
	nickname := strings.TrimSpace(input.Nickname)
	if nickname == "" {
		nickname = input.Username
	}

	model := userModel{
		Username:     input.Username,
		Nickname:     nickname,
		PasswordHash: input.PasswordHash,
		Phone:        input.Phone,
		Email:        input.Email,
		ExtJSON:      input.Ext,
		Status:       userStatusActive,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domainuser.User{}, err
	}
	return model.toDomain(), nil
}

func (r *UserRepository) UpdateByID(ctx context.Context, userID uint64, updates map[string]any) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&userModel{}).
		Where("id = ?", userID).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (m userModel) toDomain() domainuser.User {
	nickname := strings.TrimSpace(m.Nickname)
	if nickname == "" {
		nickname = m.Username
	}

	status := domainuser.StatusPending
	switch m.Status {
	case userStatusActive:
		status = domainuser.StatusActive
	case userStatusDisabled:
		status = domainuser.StatusDisabled
	}

	return domainuser.User{
		ID:       m.ID,
		Username: m.Username,
		Nickname: nickname,
		Phone:    m.Phone,
		Email:    m.Email,
		Ext:      m.ExtJSON,
		Status:   status,
	}
}

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
