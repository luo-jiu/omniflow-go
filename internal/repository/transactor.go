package repository

import (
	"context"

	pgtx "omniflow-go/internal/repository/postgres/txctx"

	"gorm.io/gorm"
)

// Transactor 定义 usecase 可依赖的事务抽象。
type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type GormTransactor = pgtx.GormTransactor

func NewTransactor(db *gorm.DB) Transactor {
	return pgtx.NewGormTransactor(db)
}
