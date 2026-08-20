package repository

import (
	"context"
	"encoding/json"
	"time"

	domainuserpreference "omniflow-go/internal/domain/userpreference"
	pgtx "omniflow-go/internal/repository/postgres/impl/txctx"
	pgmodel "omniflow-go/internal/repository/postgres/model"
	pgquery "omniflow-go/internal/repository/postgres/query"

	"gorm.io/gorm"
)

type CreateUserPreferenceInput struct {
	UserID        uint64
	Namespace     string
	Data          string
	SchemaVersion int32
	Now           time.Time
}

type UpdateUserPreferenceInput struct {
	UserID           uint64
	Namespace        string
	Data             string
	SchemaVersion    int32
	ExpectedRevision int64
	Now              time.Time
}

type UserPreferenceRepository struct {
	db *gorm.DB
}

func NewUserPreferenceRepository(db *gorm.DB) *UserPreferenceRepository {
	return &UserPreferenceRepository{db: db}
}

func (r *UserPreferenceRepository) dbWithContext(ctx context.Context) *gorm.DB {
	if tx, ok := pgtx.FromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func (r *UserPreferenceRepository) query(ctx context.Context) *pgquery.Query {
	return pgquery.Use(r.dbWithContext(ctx))
}

func (r *UserPreferenceRepository) ListByUser(
	ctx context.Context,
	userID uint64,
) ([]domainuserpreference.Preference, error) {
	q := r.query(ctx)
	rows, err := q.UserPreference.WithContext(ctx).
		Where(q.UserPreference.UserID.Eq(int64(userID))).
		Order(q.UserPreference.Namespace).
		Find()
	if err != nil {
		return nil, err
	}

	result := make([]domainuserpreference.Preference, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainUserPreference(row))
	}
	return result, nil
}

func (r *UserPreferenceRepository) FindByUserAndNamespace(
	ctx context.Context,
	userID uint64,
	namespace string,
) (domainuserpreference.Preference, error) {
	q := r.query(ctx)
	row, err := q.UserPreference.WithContext(ctx).
		Where(
			q.UserPreference.UserID.Eq(int64(userID)),
			q.UserPreference.Namespace.Eq(namespace),
		).
		First()
	if err != nil {
		return domainuserpreference.Preference{}, mapDBError(err)
	}
	return toDomainUserPreference(row), nil
}

func (r *UserPreferenceRepository) Create(
	ctx context.Context,
	input CreateUserPreferenceInput,
) (domainuserpreference.Preference, error) {
	row := &pgmodel.UserPreference{
		UserID:        int64(input.UserID),
		Namespace:     input.Namespace,
		Data:          input.Data,
		SchemaVersion: input.SchemaVersion,
		Revision:      1,
		CreatedAt:     input.Now,
		UpdatedAt:     input.Now,
	}
	if err := r.query(ctx).UserPreference.WithContext(ctx).Create(row); err != nil {
		return domainuserpreference.Preference{}, mapDBError(err)
	}
	return toDomainUserPreference(row), nil
}

func (r *UserPreferenceRepository) UpdateByRevision(
	ctx context.Context,
	input UpdateUserPreferenceInput,
) (domainuserpreference.Preference, error) {
	result := r.dbWithContext(ctx).
		Model(&pgmodel.UserPreference{}).
		Where(
			"user_id = ? AND namespace = ? AND revision = ?",
			int64(input.UserID),
			input.Namespace,
			input.ExpectedRevision,
		).
		Updates(map[string]any{
			"data":           input.Data,
			"schema_version": input.SchemaVersion,
			"revision":       gorm.Expr("revision + 1"),
			"updated_at":     input.Now,
		})
	if result.Error != nil {
		return domainuserpreference.Preference{}, mapDBError(result.Error)
	}
	if result.RowsAffected == 0 {
		return domainuserpreference.Preference{}, ErrConflict
	}
	return r.FindByUserAndNamespace(ctx, input.UserID, input.Namespace)
}

func toDomainUserPreference(row *pgmodel.UserPreference) domainuserpreference.Preference {
	data := json.RawMessage(row.Data)
	if !json.Valid(data) {
		data = json.RawMessage(`{}`)
	}
	return domainuserpreference.Preference{
		UserID:        uint64(row.UserID),
		Namespace:     row.Namespace,
		Preferences:   data,
		SchemaVersion: row.SchemaVersion,
		Revision:      row.Revision,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}
