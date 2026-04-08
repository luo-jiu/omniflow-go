package repository

import (
	"context"

	pgtx "omniflow-go/internal/repository/postgres/txctx"

	"gorm.io/gorm"
)

const (
	nodeTypeDirectory = 0
	nodeTypeFile      = 1
)

type NodeRepository struct {
	db *gorm.DB
}

type NodePathItem struct {
	ID    uint64
	Name  string
	Depth int
}

type DeleteNodeTreeResult struct {
	DeletedNodeCount int
	FileNodeCount    int64
}

func NewNodeRepository(db *gorm.DB) *NodeRepository {
	return &NodeRepository{db: db}
}

func (r *NodeRepository) WithTx(tx *gorm.DB) *NodeRepository {
	if tx == nil {
		return r
	}
	return &NodeRepository{db: tx}
}

func (r *NodeRepository) dbWithContext(ctx context.Context) *gorm.DB {
	if tx, ok := pgtx.FromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
