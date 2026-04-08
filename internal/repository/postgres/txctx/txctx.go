package txctx

import (
	"context"

	"gorm.io/gorm"
)

type txContextKey struct{}

// WithTx 将事务对象写入上下文，供仓储层自动感知并复用。
func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	if tx == nil {
		return ctx
	}
	return context.WithValue(ctx, txContextKey{}, tx)
}

// FromContext 从上下文读取事务对象。
func FromContext(ctx context.Context) (*gorm.DB, bool) {
	if ctx == nil {
		return nil, false
	}
	tx, ok := ctx.Value(txContextKey{}).(*gorm.DB)
	if !ok || tx == nil {
		return nil, false
	}
	return tx, true
}
