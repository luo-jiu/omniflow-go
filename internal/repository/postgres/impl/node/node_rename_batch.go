package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm/clause"
	domainnode "omniflow-go/internal/domain/node"
	pgmodel "omniflow-go/internal/repository/postgres/model"
)

// LockMutationOperation 将同一 actor 的 operation 请求串行化，调用方必须持有事务。
func (r *NodeRepository) LockMutationOperation(ctx context.Context, actorID, operationID string) error {
	scope := fmt.Sprintf("nodes:mutation:%s:%s", actorID, operationID)
	return r.dbWithContext(ctx).Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", scope).Error
}

// ReadMutationReceipt 仅查询 actor 自己的回执，不能跨身份枚举操作。
func (r *NodeRepository) ReadMutationReceipt(ctx context.Context, actorID, operationID string) (domainnode.MutationReceipt, error) {
	q := r.query(ctx)
	row, err := q.NodeMutationReceipt.WithContext(ctx).Where(
		q.NodeMutationReceipt.ActorID.Eq(actorID), q.NodeMutationReceipt.OperationID.Eq(operationID)).First()
	if err != nil {
		return domainnode.MutationReceipt{}, mapDBError(err)
	}
	return domainnode.MutationReceipt{ActorID: row.ActorID, OperationID: row.OperationID, Kind: row.Kind,
		LibraryID: toDomainUint64(row.LibraryID), RequestHash: row.RequestHash, Result: row.Result}, nil
}

// SaveMutationReceipt 必须在节点变更所在事务内调用，不提前发布结果。
func (r *NodeRepository) SaveMutationReceipt(ctx context.Context, receipt domainnode.MutationReceipt) error {
	q := r.query(ctx)
	return q.NodeMutationReceipt.WithContext(ctx).Create(&pgmodel.NodeMutationReceipt{
		ActorID: receipt.ActorID, OperationID: receipt.OperationID, LibraryID: toPGInt64(receipt.LibraryID),
		Kind: receipt.Kind, RequestHash: receipt.RequestHash, Result: receipt.Result,
	})
}

// LockRenameNodes 按固定 ID 顺序锁定本库未删除节点，避免批次之间产生锁顺序反转。
func (r *NodeRepository) LockRenameNodes(ctx context.Context, libraryID uint64, ids []uint64) ([]domainnode.Node, error) {
	q := r.query(ctx)
	rows, err := q.Node.WithContext(ctx).Where(q.Node.LibraryID.Eq(toPGInt64(libraryID)), q.Node.ID.In(toPGInt64Slice(ids)...)).
		Order(q.Node.ID).Clauses(clause.Locking{Strength: "UPDATE"}).Find()
	if err != nil {
		return nil, mapDBError(err)
	}
	nodes := make([]domainnode.Node, 0, len(rows))
	for _, row := range rows {
		nodes = append(nodes, toDomainNodeModel(row))
	}
	return nodes, nil
}

// RenameNameTaken 检查可见名称，包括文件与目录之间的冲突。
func (r *NodeRepository) RenameNameTaken(ctx context.Context, libraryID, parentID, nodeID uint64, name, ext string) (bool, error) {
	return r.hasDuplicateVisibleName(ctx, name, ext, domainnode.TypeFile, parentID, libraryID, nodeID)
}

// WriteLockedRename 修改已锁定节点并读回触发器维护的真实 updated_at。
func (r *NodeRepository) WriteLockedRename(ctx context.Context, libraryID, nodeID uint64, name string) (domainnode.Node, error) {
	q := r.query(ctx)
	info, err := q.Node.WithContext(ctx).Where(q.Node.LibraryID.Eq(toPGInt64(libraryID)), q.Node.ID.Eq(toPGInt64(nodeID))).
		Updates(map[string]any{"name": name, "updated_at": time.Now().UTC()})
	if err != nil {
		return domainnode.Node{}, mapDBError(err)
	}
	if info.RowsAffected != 1 {
		return domainnode.Node{}, ErrNotFound
	}
	row, err := r.findNodeModel(ctx, nodeID, libraryID)
	if err != nil {
		return domainnode.Node{}, err
	}
	return toDomainNodeModel(row), nil
}
