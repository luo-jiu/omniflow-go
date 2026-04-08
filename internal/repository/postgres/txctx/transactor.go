package txctx

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var (
	ErrTransactorNotConfigured = errors.New("gorm transactor: db is nil")
	ErrTxFuncRequired          = errors.New("gorm transactor: fn is required")
)

// GormTransactor 统一管理事务开启、提交与回滚。
type GormTransactor struct {
	db *gorm.DB
}

func NewGormTransactor(db *gorm.DB) *GormTransactor {
	return &GormTransactor{db: db}
}

// WithinTx 在事务中执行 fn，并把 tx 注入到返回的 context。
func (t *GormTransactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if t == nil || t.db == nil {
		return ErrTransactorNotConfigured
	}
	if fn == nil {
		return ErrTxFuncRequired
	}

	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(WithTx(ctx, tx))
	})
}
