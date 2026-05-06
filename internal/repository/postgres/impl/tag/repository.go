package repository

import (
	"context"
	"sort"
	"strings"
	"time"

	domaintag "omniflow-go/internal/domain/tag"
	pgtx "omniflow-go/internal/repository/postgres/impl/txctx"

	"github.com/samber/lo"
	"gorm.io/gorm"
)

type tagEntity struct {
	ID           int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Name         string         `gorm:"column:name"`
	Type         string         `gorm:"column:type"`
	Scope        string         `gorm:"column:scope"`
	Dimension    string         `gorm:"column:dimension"`
	ResourceKind *string        `gorm:"column:resource_kind"`
	TargetKey    *string        `gorm:"column:target_key"`
	OwnerUserID  *int64         `gorm:"column:owner_user_id"`
	Color        string         `gorm:"column:color"`
	TextColor    *string        `gorm:"column:text_color"`
	SortOrder    int32          `gorm:"column:sort_order"`
	Enabled      bool           `gorm:"column:enabled"`
	Description  *string        `gorm:"column:description"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at"`
	CreatedAt    time.Time      `gorm:"column:created_at"`
	UpdatedAt    time.Time      `gorm:"column:updated_at"`
}

func (tagEntity) TableName() string {
	return "tags"
}

type CreateTagInput struct {
	Name         string
	Type         string
	Scope        string
	Dimension    string
	ResourceKind *string
	TargetKey    *string
	OwnerUserID  uint64
	Color        string
	TextColor    *string
	SortOrder    int
	Enabled      int
	Description  *string
}

type UpdateTagInput struct {
	Name         string
	Type         string
	Scope        string
	Dimension    string
	ResourceKind *string
	TargetKey    *string
	Color        string
	TextColor    *string
	SortOrder    int
	Enabled      int
	Description  *string
}

type ListTagsFilter struct {
	Type         *string
	Scope        *string
	Dimension    *string
	ResourceKind *string
}

type TagNameScope struct {
	Type         string
	Scope        string
	Dimension    string
	ResourceKind *string
	Name         string
}

type TagRepository struct {
	db *gorm.DB
}

func NewTagRepository(db *gorm.DB) *TagRepository {
	return &TagRepository{db: db}
}

func (r *TagRepository) WithTx(tx *gorm.DB) *TagRepository {
	if tx == nil {
		return r
	}
	return &TagRepository{db: tx}
}

// LockScopes 通过事务级 advisory lock 串行化同一业务唯一键范围的并发写入。
func (r *TagRepository) LockScopes(ctx context.Context, scopes ...string) error {
	if len(scopes) == 0 {
		return nil
	}

	uniqueScopes := lo.Uniq(lo.FilterMap(scopes, func(scope string, _ int) (string, bool) {
		clean := strings.TrimSpace(scope)
		if clean == "" {
			return "", false
		}
		return clean, true
	}))
	if len(uniqueScopes) == 0 {
		return nil
	}

	// 固定锁顺序，避免多锁场景下死锁。
	sort.Strings(uniqueScopes)

	db := r.dbWithContext(ctx)
	for _, scope := range uniqueScopes {
		if err := db.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", scope).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *TagRepository) dbWithContext(ctx context.Context) *gorm.DB {
	if tx, ok := pgtx.FromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func (r *TagRepository) ListByOwnerAndFilter(ctx context.Context, ownerUserID uint64, filter ListTagsFilter) ([]domaintag.Tag, error) {
	db := r.dbWithContext(ctx)
	query := db.Model(&tagEntity{}).
		Where("(owner_user_id = ? OR owner_user_id IS NULL)", toPGInt64(ownerUserID))
	if filter.Type != nil && *filter.Type != "" {
		query = query.Where("type = ?", *filter.Type)
	}
	if filter.Scope != nil && *filter.Scope != "" {
		query = query.Where("scope = ?", *filter.Scope)
	}
	if filter.Dimension != nil && *filter.Dimension != "" {
		query = query.Where("dimension = ?", *filter.Dimension)
	}
	if filter.ResourceKind != nil && *filter.ResourceKind != "" {
		query = query.Where("resource_kind = ?", *filter.ResourceKind)
	}

	var rows []*tagEntity
	if err := query.
		Order("scope ASC").
		Order("type ASC").
		Order("dimension ASC").
		Order("resource_kind ASC").
		Order("sort_order ASC").
		Order("target_key ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := lo.Map(rows, func(row *tagEntity, _ int) domaintag.Tag {
		return toDomainTag(row)
	})
	return result, nil
}

func (r *TagRepository) Create(ctx context.Context, input CreateTagInput) (domaintag.Tag, error) {
	owner := toPGInt64(input.OwnerUserID)
	row := &tagEntity{
		Name:         input.Name,
		Type:         input.Type,
		Scope:        input.Scope,
		Dimension:    input.Dimension,
		ResourceKind: input.ResourceKind,
		TargetKey:    input.TargetKey,
		OwnerUserID:  &owner,
		Color:        input.Color,
		TextColor:    input.TextColor,
		SortOrder:    int32(input.SortOrder),
		Enabled:      toDBEnabled(input.Enabled),
		Description:  input.Description,
	}

	if err := r.dbWithContext(ctx).Create(row).Error; err != nil {
		return domaintag.Tag{}, mapDBError(err)
	}
	return toDomainTag(row), nil
}

func (r *TagRepository) FindOwnerByID(ctx context.Context, id, ownerUserID uint64) (domaintag.Tag, error) {
	row, err := r.findOwnerTagEntityByID(ctx, id, ownerUserID)
	if err != nil {
		return domaintag.Tag{}, err
	}
	return toDomainTag(row), nil
}

func (r *TagRepository) UpdateOwnerByID(ctx context.Context, id, ownerUserID uint64, input UpdateTagInput) (domaintag.Tag, error) {
	updates := map[string]any{
		"name":          input.Name,
		"type":          input.Type,
		"scope":         input.Scope,
		"dimension":     input.Dimension,
		"resource_kind": input.ResourceKind,
		"target_key":    input.TargetKey,
		"color":         input.Color,
		"text_color":    input.TextColor,
		"sort_order":    int32(input.SortOrder),
		"enabled":       toDBEnabled(input.Enabled),
		"description":   input.Description,
		"updated_at":    time.Now().UTC(),
	}

	result := r.dbWithContext(ctx).
		Model(&tagEntity{}).
		Where("id = ? AND owner_user_id = ?", toPGInt64(id), toPGInt64(ownerUserID)).
		Updates(updates)
	if result.Error != nil {
		return domaintag.Tag{}, mapDBError(result.Error)
	}
	if result.RowsAffected == 0 {
		return domaintag.Tag{}, ErrNotFound
	}
	return r.FindOwnerByID(ctx, id, ownerUserID)
}

func (r *TagRepository) SoftDeleteOwnerByID(ctx context.Context, id, ownerUserID uint64) (bool, error) {
	now := time.Now().UTC()
	result := r.dbWithContext(ctx).
		Model(&tagEntity{}).
		Where("id = ? AND owner_user_id = ?", toPGInt64(id), toPGInt64(ownerUserID)).
		Updates(map[string]any{
			"deleted_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *TagRepository) ExistsName(ctx context.Context, ownerUserID uint64, scope TagNameScope, excludeID uint64) (bool, error) {
	query := r.dbWithContext(ctx).
		Model(&tagEntity{}).
		Where(
			"owner_user_id = ? AND type = ? AND scope = ? AND dimension = ? AND name = ?",
			toPGInt64(ownerUserID),
			scope.Type,
			scope.Scope,
			scope.Dimension,
			scope.Name,
		)
	if scope.ResourceKind == nil || *scope.ResourceKind == "" {
		query = query.Where("resource_kind IS NULL")
	} else {
		query = query.Where("resource_kind = ?", *scope.ResourceKind)
	}
	if excludeID > 0 {
		query = query.Where("id <> ?", toPGInt64(excludeID))
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *TagRepository) ExistsTargetKey(ctx context.Context, ownerUserID uint64, tagType string, targetKey *string, excludeID uint64) (bool, error) {
	if targetKey == nil || *targetKey == "" {
		return false, nil
	}

	query := r.dbWithContext(ctx).
		Model(&tagEntity{}).
		Where(
			"owner_user_id = ? AND type = ? AND target_key = ?",
			toPGInt64(ownerUserID),
			tagType,
			*targetKey,
		)
	if excludeID > 0 {
		query = query.Where("id <> ?", toPGInt64(excludeID))
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *TagRepository) findOwnerTagEntityByID(ctx context.Context, id, ownerUserID uint64) (*tagEntity, error) {
	var row tagEntity
	if err := r.dbWithContext(ctx).
		Model(&tagEntity{}).
		Where("id = ? AND owner_user_id = ?", toPGInt64(id), toPGInt64(ownerUserID)).
		First(&row).Error; err != nil {
		return nil, mapDBError(err)
	}
	return &row, nil
}

func toDomainTag(row *tagEntity) domaintag.Tag {
	if row == nil {
		return domaintag.Tag{}
	}

	var ownerUserID *uint64
	if row.OwnerUserID != nil && *row.OwnerUserID > 0 {
		value := uint64(*row.OwnerUserID)
		ownerUserID = &value
	}

	createdAt := row.CreatedAt
	updatedAt := row.UpdatedAt
	return domaintag.Tag{
		ID:           toDomainUint64(row.ID),
		Name:         row.Name,
		Type:         row.Type,
		Scope:        row.Scope,
		Dimension:    row.Dimension,
		ResourceKind: row.ResourceKind,
		TargetKey:    row.TargetKey,
		OwnerUserID:  ownerUserID,
		Color:        row.Color,
		TextColor:    row.TextColor,
		SortOrder:    int(row.SortOrder),
		Enabled:      toAPIEnabled(row.Enabled),
		Description:  row.Description,
		CreatedAt:    &createdAt,
		UpdatedAt:    &updatedAt,
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

func toDBEnabled(value int) bool {
	return value == 1
}

func toAPIEnabled(value bool) int {
	if value {
		return 1
	}
	return 0
}
