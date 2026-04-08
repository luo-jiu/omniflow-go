package repository

import (
	"context"
	"strings"

	domainuser "omniflow-go/internal/domain/user"
	pgtx "omniflow-go/internal/repository/postgres/impl/txctx"
	pgmodel "omniflow-go/internal/repository/postgres/model"
	pgquery "omniflow-go/internal/repository/postgres/query"

	"gorm.io/gorm"
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

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) WithTx(tx *gorm.DB) *UserRepository {
	if tx == nil {
		return r
	}
	return &UserRepository{db: tx}
}

func (r *UserRepository) dbWithContext(ctx context.Context) *gorm.DB {
	if tx, ok := pgtx.FromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func (r *UserRepository) query(ctx context.Context) *pgquery.Query {
	return pgquery.Use(r.dbWithContext(ctx))
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (domainuser.User, error) {
	q := r.query(ctx)

	row, err := q.User.WithContext(ctx).
		Where(q.User.Username.Eq(username)).
		First()
	if err != nil {
		return domainuser.User{}, mapDBError(err)
	}
	return toDomainUserModel(row), nil
}

func (r *UserRepository) FindByID(ctx context.Context, userID uint64) (domainuser.User, error) {
	q := r.query(ctx)

	row, err := q.User.WithContext(ctx).
		Where(q.User.ID.Eq(int64(userID))).
		First()
	if err != nil {
		return domainuser.User{}, mapDBError(err)
	}
	return toDomainUserModel(row), nil
}

func (r *UserRepository) FindAuthByID(ctx context.Context, userID uint64) (UserAuth, error) {
	q := r.query(ctx)

	row, err := q.User.WithContext(ctx).
		Where(q.User.ID.Eq(int64(userID))).
		First()
	if err != nil {
		return UserAuth{}, mapDBError(err)
	}
	return UserAuth{
		User:         toDomainUserModel(row),
		PasswordHash: row.PasswordHash,
	}, nil
}

func (r *UserRepository) FindActiveByUsername(ctx context.Context, username string) (UserAuth, error) {
	q := r.query(ctx)

	row, err := q.User.WithContext(ctx).
		Where(
			q.User.Username.Eq(username),
			q.User.Status.Eq(int16(userStatusActive)),
		).
		First()
	if err != nil {
		return UserAuth{}, mapDBError(err)
	}
	return UserAuth{
		User:         toDomainUserModel(row),
		PasswordHash: row.PasswordHash,
	}, nil
}

func (r *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	q := r.query(ctx)

	count, err := q.User.WithContext(ctx).
		Where(q.User.Username.Eq(username)).
		Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepository) Create(ctx context.Context, input CreateUserInput) (domainuser.User, error) {
	nickname := strings.TrimSpace(input.Nickname)
	if nickname == "" {
		nickname = input.Username
	}

	ext := strings.TrimSpace(input.Ext)
	if ext == "" {
		ext = "{}"
	}

	row := &pgmodel.User{
		Username:     input.Username,
		Nickname:     nullableString(nickname),
		PasswordHash: input.PasswordHash,
		Phone:        nullableString(input.Phone),
		Email:        nullableString(input.Email),
		Ext:          ext,
		Status:       int16(userStatusActive),
	}

	q := r.query(ctx)
	if err := q.User.WithContext(ctx).Create(row); err != nil {
		return domainuser.User{}, err
	}
	return toDomainUserModel(row), nil
}

func (r *UserRepository) UpdateByID(ctx context.Context, userID uint64, updates map[string]any) (bool, error) {
	q := r.query(ctx)

	info, err := q.User.WithContext(ctx).
		Where(q.User.ID.Eq(int64(userID))).
		Updates(updates)
	if err != nil {
		return false, err
	}
	return info.RowsAffected > 0, nil
}
