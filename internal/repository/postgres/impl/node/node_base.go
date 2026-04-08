package repository

import (
	"context"

	pgtx "omniflow-go/internal/repository/postgres/impl/txctx"
	pgmodel "omniflow-go/internal/repository/postgres/model"
	pgquery "omniflow-go/internal/repository/postgres/query"

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

func (r *NodeRepository) query(ctx context.Context) *pgquery.Query {
	return pgquery.Use(r.dbWithContext(ctx))
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

func toPGInt64Slice(values []uint64) []int64 {
	if len(values) == 0 {
		return []int64{}
	}

	result := make([]int64, 0, len(values))
	for _, item := range values {
		result = append(result, toPGInt64(item))
	}
	return result
}

func toDomainUint64Slice(values []int64) []uint64 {
	if len(values) == 0 {
		return []uint64{}
	}

	result := make([]uint64, 0, len(values))
	for _, item := range values {
		if item <= 0 {
			continue
		}
		result = append(result, uint64(item))
	}
	return result
}

func normalizePGParentID(parentID uint64) *int64 {
	if parentID == 0 {
		return nil
	}
	value := toPGInt64(parentID)
	return &value
}

func parentIDValue(parentID *int64) uint64 {
	if parentID == nil {
		return 0
	}
	return toDomainUint64(*parentID)
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	result := value
	return &result
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (r *NodeRepository) findNodeModel(ctx context.Context, nodeID, libraryID uint64) (*pgmodel.Node, error) {
	q := r.query(ctx)
	row, err := q.Node.WithContext(ctx).
		Where(
			q.Node.ID.Eq(toPGInt64(nodeID)),
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
		).
		First()
	if err != nil {
		return nil, mapDBError(err)
	}
	return row, nil
}

func (r *NodeRepository) findNodeModelByID(ctx context.Context, nodeID uint64) (*pgmodel.Node, error) {
	q := r.query(ctx)
	row, err := q.Node.WithContext(ctx).
		Where(q.Node.ID.Eq(toPGInt64(nodeID))).
		First()
	if err != nil {
		return nil, mapDBError(err)
	}
	return row, nil
}

func (r *NodeRepository) applyParentCondition(do pgquery.INodeDo, q *pgquery.Query, parentID uint64) pgquery.INodeDo {
	if parentID == 0 {
		return do.Where(q.Node.ParentID.IsNull())
	}
	return do.Where(q.Node.ParentID.Eq(toPGInt64(parentID)))
}
