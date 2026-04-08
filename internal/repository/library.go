package repository

import (
	"context"
	"time"

	domainlibrary "omniflow-go/internal/domain/library"

	"gorm.io/gorm"
)

type LibraryRepository struct {
	db *gorm.DB
}

type libraryModel struct {
	ID        uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    uint64         `gorm:"column:user_id"`
	Name      string         `gorm:"column:name"`
	Starred   bool           `gorm:"column:starred"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (libraryModel) TableName() string {
	return "libraries"
}

func NewLibraryRepository(db *gorm.DB) *LibraryRepository {
	return &LibraryRepository{db: db}
}

func (r *LibraryRepository) WithTx(tx *gorm.DB) *LibraryRepository {
	if tx == nil {
		return r
	}
	return &LibraryRepository{db: tx}
}

func (r *LibraryRepository) ScrollByUser(ctx context.Context, userID, lastID uint64, size int) ([]domainlibrary.Library, error) {
	tx := r.db.WithContext(ctx).Model(&libraryModel{}).Where("user_id = ?", userID).Order("id ASC").Limit(size)
	if lastID > 0 {
		tx = tx.Where("id > ?", lastID)
	}

	var rows []libraryModel
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]domainlibrary.Library, 0, len(rows))
	for _, item := range rows {
		result = append(result, item.toDomain())
	}
	return result, nil
}

func (r *LibraryRepository) Create(ctx context.Context, userID uint64, name string) (domainlibrary.Library, error) {
	row := libraryModel{
		UserID: userID,
		Name:   name,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domainlibrary.Library{}, err
	}
	return row.toDomain(), nil
}

func (r *LibraryRepository) UpdateName(ctx context.Context, id, userID uint64, name string, updatedAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&libraryModel{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"name":       name,
			"updated_at": updatedAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *LibraryRepository) SoftDelete(ctx context.Context, id, userID uint64, deletedAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&libraryModel{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("deleted_at", deletedAt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *LibraryRepository) FindByID(ctx context.Context, id uint64) (domainlibrary.Library, error) {
	var row libraryModel
	if err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domainlibrary.Library{}, mapDBError(err)
	}
	return row.toDomain(), nil
}

func (m libraryModel) toDomain() domainlibrary.Library {
	return domainlibrary.Library{
		ID:      m.ID,
		UserID:  m.UserID,
		Name:    m.Name,
		Starred: m.Starred,
	}
}
