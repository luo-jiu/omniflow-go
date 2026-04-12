package repository

import (
	"context"
	"time"

	domainbrowserbookmark "omniflow-go/internal/domain/browserbookmark"
	pgtx "omniflow-go/internal/repository/postgres/impl/txctx"
	"omniflow-go/internal/repository/postgres/model"

	"gorm.io/gorm"
)

const bookmarkSortOrderStep = 1000

type CreateBrowserBookmarkInput struct {
	OwnerUserID uint64
	ParentID    *uint64
	Kind        string
	Title       string
	URL         *string
	URLMatchKey *string
	IconURL     *string
	SortOrder   int
}

type UpdateBrowserBookmarkInput struct {
	Title       *string
	URL         *string
	URLMatchKey *string
	IconURL     *string
}

type BrowserBookmarkSortOrder struct {
	ID        uint64
	SortOrder int
}

type BrowserBookmarkRepository struct {
	db *gorm.DB
}

func NewBrowserBookmarkRepository(db *gorm.DB) *BrowserBookmarkRepository {
	return &BrowserBookmarkRepository{db: db}
}

func (r *BrowserBookmarkRepository) WithTx(tx *gorm.DB) *BrowserBookmarkRepository {
	if tx == nil {
		return r
	}
	return &BrowserBookmarkRepository{db: tx}
}

func (r *BrowserBookmarkRepository) dbWithContext(ctx context.Context) *gorm.DB {
	if tx, ok := pgtx.FromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func (r *BrowserBookmarkRepository) ListByOwner(ctx context.Context, ownerUserID uint64) ([]domainbrowserbookmark.BrowserBookmark, error) {
	var rows []*model.BrowserBookmark
	if err := r.dbWithContext(ctx).
		Model(&model.BrowserBookmark{}).
		Where("owner_user_id = ?", toPGInt64(ownerUserID)).
		Order("sort_order ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]domainbrowserbookmark.BrowserBookmark, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainBrowserBookmark(row))
	}
	return result, nil
}

func (r *BrowserBookmarkRepository) ListSiblings(
	ctx context.Context,
	ownerUserID uint64,
	parentID *uint64,
	excludeID uint64,
) ([]domainbrowserbookmark.BrowserBookmark, error) {
	query := r.dbWithContext(ctx).
		Model(&model.BrowserBookmark{}).
		Where("owner_user_id = ?", toPGInt64(ownerUserID))
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", toPGInt64(*parentID))
	}
	if excludeID > 0 {
		query = query.Where("id <> ?", toPGInt64(excludeID))
	}

	var rows []*model.BrowserBookmark
	if err := query.Order("sort_order ASC").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]domainbrowserbookmark.BrowserBookmark, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainBrowserBookmark(row))
	}
	return result, nil
}

func (r *BrowserBookmarkRepository) FindOwnerByID(
	ctx context.Context,
	id, ownerUserID uint64,
) (domainbrowserbookmark.BrowserBookmark, error) {
	var row model.BrowserBookmark
	if err := r.dbWithContext(ctx).
		Model(&model.BrowserBookmark{}).
		Where("id = ? AND owner_user_id = ?", toPGInt64(id), toPGInt64(ownerUserID)).
		First(&row).Error; err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, mapDBError(err)
	}
	return toDomainBrowserBookmark(&row), nil
}

func (r *BrowserBookmarkRepository) FindFirstURLByMatchKey(
	ctx context.Context,
	ownerUserID uint64,
	urlMatchKey string,
) (domainbrowserbookmark.BrowserBookmark, error) {
	var row model.BrowserBookmark
	if err := r.dbWithContext(ctx).
		Model(&model.BrowserBookmark{}).
		Where(
			"owner_user_id = ? AND kind = ? AND url_match_key = ?",
			toPGInt64(ownerUserID),
			domainbrowserbookmark.KindURL,
			urlMatchKey,
		).
		Order("id ASC").
		First(&row).Error; err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, mapDBError(err)
	}
	return toDomainBrowserBookmark(&row), nil
}

func (r *BrowserBookmarkRepository) NextSortOrder(ctx context.Context, ownerUserID uint64, parentID *uint64) (int, error) {
	query := r.dbWithContext(ctx).
		Model(&model.BrowserBookmark{}).
		Where("owner_user_id = ?", toPGInt64(ownerUserID))
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", toPGInt64(*parentID))
	}

	var maxOrder int
	if err := query.Select("COALESCE(MAX(sort_order), 0)").Scan(&maxOrder).Error; err != nil {
		return 0, err
	}
	return maxOrder + bookmarkSortOrderStep, nil
}

func (r *BrowserBookmarkRepository) Create(
	ctx context.Context,
	input CreateBrowserBookmarkInput,
) (domainbrowserbookmark.BrowserBookmark, error) {
	row := &model.BrowserBookmark{
		OwnerUserID: toPGInt64(input.OwnerUserID),
		ParentID:    toPGInt64Ptr(input.ParentID),
		Kind:        input.Kind,
		Title:       input.Title,
		URL:         input.URL,
		URLMatchKey: input.URLMatchKey,
		IconURL:     input.IconURL,
		SortOrder:   int32(input.SortOrder),
	}
	if err := r.dbWithContext(ctx).Create(row).Error; err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, mapDBError(err)
	}
	return toDomainBrowserBookmark(row), nil
}

func (r *BrowserBookmarkRepository) UpdateOwnerByID(
	ctx context.Context,
	id, ownerUserID uint64,
	input UpdateBrowserBookmarkInput,
) (domainbrowserbookmark.BrowserBookmark, error) {
	updates := map[string]any{
		"updated_at": time.Now().UTC(),
	}
	if input.Title != nil {
		updates["title"] = *input.Title
	}
	if input.URL != nil {
		updates["url"] = *input.URL
	}
	if input.URLMatchKey != nil {
		updates["url_match_key"] = *input.URLMatchKey
	}
	if input.IconURL != nil {
		if *input.IconURL == "" {
			updates["icon_url"] = nil
		} else {
			updates["icon_url"] = *input.IconURL
		}
	}

	result := r.dbWithContext(ctx).
		Model(&model.BrowserBookmark{}).
		Where("id = ? AND owner_user_id = ?", toPGInt64(id), toPGInt64(ownerUserID)).
		Updates(updates)
	if result.Error != nil {
		return domainbrowserbookmark.BrowserBookmark{}, mapDBError(result.Error)
	}
	if result.RowsAffected == 0 {
		return domainbrowserbookmark.BrowserBookmark{}, ErrNotFound
	}
	return r.FindOwnerByID(ctx, id, ownerUserID)
}

func (r *BrowserBookmarkRepository) MoveOwnerByID(
	ctx context.Context,
	id, ownerUserID uint64,
	parentID *uint64,
) error {
	updates := map[string]any{
		"parent_id":  toPGInt64Ptr(parentID),
		"updated_at": time.Now().UTC(),
	}
	result := r.dbWithContext(ctx).
		Model(&model.BrowserBookmark{}).
		Where("id = ? AND owner_user_id = ?", toPGInt64(id), toPGInt64(ownerUserID)).
		Updates(updates)
	if result.Error != nil {
		return mapDBError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BrowserBookmarkRepository) UpdateSortOrders(
	ctx context.Context,
	ownerUserID uint64,
	orders []BrowserBookmarkSortOrder,
) error {
	if len(orders) == 0 {
		return nil
	}
	db := r.dbWithContext(ctx)
	now := time.Now().UTC()
	for _, item := range orders {
		result := db.Model(&model.BrowserBookmark{}).
			Where("id = ? AND owner_user_id = ?", toPGInt64(item.ID), toPGInt64(ownerUserID)).
			Updates(map[string]any{
				"sort_order": int32(item.SortOrder),
				"updated_at": now,
			})
		if result.Error != nil {
			return mapDBError(result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
	}
	return nil
}

func (r *BrowserBookmarkRepository) SoftDeleteTreeOwnerByID(ctx context.Context, id, ownerUserID uint64) (bool, error) {
	now := time.Now().UTC()
	result := r.dbWithContext(ctx).Exec(`
WITH RECURSIVE target AS (
  SELECT id
  FROM browser_bookmarks
  WHERE id = ? AND owner_user_id = ? AND deleted_at IS NULL
  UNION ALL
  SELECT child.id
  FROM browser_bookmarks child
  JOIN target parent ON child.parent_id = parent.id
  WHERE child.owner_user_id = ? AND child.deleted_at IS NULL
)
UPDATE browser_bookmarks b
SET deleted_at = ?, updated_at = ?
FROM target
WHERE b.id = target.id AND b.deleted_at IS NULL
`, toPGInt64(id), toPGInt64(ownerUserID), toPGInt64(ownerUserID), now, now)
	if result.Error != nil {
		return false, mapDBError(result.Error)
	}
	return result.RowsAffected > 0, nil
}

func toDomainBrowserBookmark(row *model.BrowserBookmark) domainbrowserbookmark.BrowserBookmark {
	if row == nil {
		return domainbrowserbookmark.BrowserBookmark{}
	}
	return domainbrowserbookmark.BrowserBookmark{
		ID:          toDomainUint64(row.ID),
		OwnerUserID: toDomainUint64(row.OwnerUserID),
		ParentID:    toDomainUint64Ptr(row.ParentID),
		Kind:        row.Kind,
		Title:       row.Title,
		URL:         row.URL,
		URLMatchKey: row.URLMatchKey,
		IconURL:     row.IconURL,
		SortOrder:   int(row.SortOrder),
		CreatedAt:   timePtrOrNil(row.CreatedAt),
		UpdatedAt:   timePtrOrNil(row.UpdatedAt),
	}
}

func toPGInt64(value uint64) int64 {
	return int64(value)
}

func toPGInt64Ptr(value *uint64) *int64 {
	if value == nil || *value == 0 {
		return nil
	}
	converted := int64(*value)
	return &converted
}

func toDomainUint64(value int64) uint64 {
	if value <= 0 {
		return 0
	}
	return uint64(value)
}

func toDomainUint64Ptr(value *int64) *uint64 {
	if value == nil || *value <= 0 {
		return nil
	}
	converted := uint64(*value)
	return &converted
}

func timePtrOrNil(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copyValue := value
	return &copyValue
}
