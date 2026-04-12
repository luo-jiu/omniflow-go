package repository

import (
	"context"
	"time"

	domainbrowserfilemapping "omniflow-go/internal/domain/browserfilemapping"
	pgtx "omniflow-go/internal/repository/postgres/impl/txctx"
	"omniflow-go/internal/repository/postgres/model"

	"gorm.io/gorm"
)

type CreateBrowserFileMappingInput struct {
	FileExt     string
	SiteURL     string
	OwnerUserID uint64
}

type UpdateBrowserFileMappingInput struct {
	FileExt string
	SiteURL string
}

type BrowserFileMappingRepository struct {
	db *gorm.DB
}

func NewBrowserFileMappingRepository(db *gorm.DB) *BrowserFileMappingRepository {
	return &BrowserFileMappingRepository{db: db}
}

func (r *BrowserFileMappingRepository) WithTx(tx *gorm.DB) *BrowserFileMappingRepository {
	if tx == nil {
		return r
	}
	return &BrowserFileMappingRepository{db: tx}
}

func (r *BrowserFileMappingRepository) dbWithContext(ctx context.Context) *gorm.DB {
	if tx, ok := pgtx.FromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func (r *BrowserFileMappingRepository) ListByOwner(ctx context.Context, ownerUserID uint64) ([]domainbrowserfilemapping.BrowserFileMapping, error) {
	var rows []*model.BrowserFileMapping
	if err := r.dbWithContext(ctx).
		Model(&model.BrowserFileMapping{}).
		Where("owner_user_id = ?", toPGInt64(ownerUserID)).
		Order("file_ext ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]domainbrowserfilemapping.BrowserFileMapping, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainBrowserFileMapping(row))
	}
	return result, nil
}

func (r *BrowserFileMappingRepository) FindOwnerByID(ctx context.Context, id, ownerUserID uint64) (domainbrowserfilemapping.BrowserFileMapping, error) {
	row, err := r.findOwnerEntityByID(ctx, id, ownerUserID)
	if err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}
	return toDomainBrowserFileMapping(row), nil
}

func (r *BrowserFileMappingRepository) FindByOwnerAndExt(
	ctx context.Context,
	ownerUserID uint64,
	fileExt string,
) (domainbrowserfilemapping.BrowserFileMapping, error) {
	var row model.BrowserFileMapping
	if err := r.dbWithContext(ctx).
		Model(&model.BrowserFileMapping{}).
		Where(
			"owner_user_id = ? AND file_ext = ?",
			toPGInt64(ownerUserID),
			fileExt,
		).
		First(&row).Error; err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, mapDBError(err)
	}
	return toDomainBrowserFileMapping(&row), nil
}

func (r *BrowserFileMappingRepository) Create(
	ctx context.Context,
	input CreateBrowserFileMappingInput,
) (domainbrowserfilemapping.BrowserFileMapping, error) {
	row := &model.BrowserFileMapping{
		FileExt:     input.FileExt,
		SiteURL:     input.SiteURL,
		OwnerUserID: toPGInt64(input.OwnerUserID),
	}
	if err := r.dbWithContext(ctx).Create(row).Error; err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, mapDBError(err)
	}
	return toDomainBrowserFileMapping(row), nil
}

func (r *BrowserFileMappingRepository) UpdateOwnerByID(
	ctx context.Context,
	id, ownerUserID uint64,
	input UpdateBrowserFileMappingInput,
) (domainbrowserfilemapping.BrowserFileMapping, error) {
	updates := map[string]any{
		"file_ext":   input.FileExt,
		"site_url":   input.SiteURL,
		"updated_at": time.Now().UTC(),
	}

	result := r.dbWithContext(ctx).
		Model(&model.BrowserFileMapping{}).
		Where("id = ? AND owner_user_id = ?", toPGInt64(id), toPGInt64(ownerUserID)).
		Updates(updates)
	if result.Error != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, mapDBError(result.Error)
	}
	if result.RowsAffected == 0 {
		return domainbrowserfilemapping.BrowserFileMapping{}, ErrNotFound
	}
	return r.FindOwnerByID(ctx, id, ownerUserID)
}

func (r *BrowserFileMappingRepository) SoftDeleteOwnerByID(ctx context.Context, id, ownerUserID uint64) (bool, error) {
	now := time.Now().UTC()
	result := r.dbWithContext(ctx).
		Model(&model.BrowserFileMapping{}).
		Where("id = ? AND owner_user_id = ?", toPGInt64(id), toPGInt64(ownerUserID)).
		Updates(map[string]any{
			"deleted_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return false, mapDBError(result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (r *BrowserFileMappingRepository) ExistsFileExt(ctx context.Context, ownerUserID uint64, fileExt string, excludeID uint64) (bool, error) {
	query := r.dbWithContext(ctx).
		Model(&model.BrowserFileMapping{}).
		Where("owner_user_id = ? AND file_ext = ?", toPGInt64(ownerUserID), fileExt)
	if excludeID > 0 {
		query = query.Where("id <> ?", toPGInt64(excludeID))
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *BrowserFileMappingRepository) findOwnerEntityByID(ctx context.Context, id, ownerUserID uint64) (*model.BrowserFileMapping, error) {
	var row model.BrowserFileMapping
	if err := r.dbWithContext(ctx).
		Model(&model.BrowserFileMapping{}).
		Where("id = ? AND owner_user_id = ?", toPGInt64(id), toPGInt64(ownerUserID)).
		First(&row).Error; err != nil {
		return nil, mapDBError(err)
	}
	return &row, nil
}

func toDomainBrowserFileMapping(row *model.BrowserFileMapping) domainbrowserfilemapping.BrowserFileMapping {
	if row == nil {
		return domainbrowserfilemapping.BrowserFileMapping{}
	}
	ownerUserID := toDomainUint64(row.OwnerUserID)
	return domainbrowserfilemapping.BrowserFileMapping{
		ID:          toDomainUint64(row.ID),
		FileExt:     row.FileExt,
		SiteURL:     row.SiteURL,
		OwnerUserID: ownerUserID,
		CreatedAt:   timePtrOrNil(row.CreatedAt),
		UpdatedAt:   timePtrOrNil(row.UpdatedAt),
	}
}

func toPGInt64(value uint64) int64 {
	return int64(value)
}

func toDomainUint64(value int64) uint64 {
	if value <= 0 {
		return 0
	}
	return uint64(value)
}

func timePtrOrNil(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copyValue := value
	return &copyValue
}
