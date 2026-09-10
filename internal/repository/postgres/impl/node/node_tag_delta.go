package repository

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm/clause"
	domainnode "omniflow-go/internal/domain/node"
	pgmodel "omniflow-go/internal/repository/postgres/model"
)

// ReadNodeTagState 读取正式关系；调用方事务内持有节点共享锁以取得一致快照。
func (r *NodeRepository) ReadNodeTagState(ctx context.Context, nodeID, libraryID uint64) (domainnode.TagState, error) {
	q := r.query(ctx)
	n, err := q.Node.WithContext(ctx).Where(q.Node.ID.Eq(toPGInt64(nodeID)), q.Node.LibraryID.Eq(toPGInt64(libraryID))).
		Clauses(clause.Locking{Strength: "SHARE"}).First()
	if err != nil {
		return domainnode.TagState{}, mapDBError(err)
	}
	return r.nodeTagState(ctx, n)
}

func (r *NodeRepository) nodeTagState(ctx context.Context, n *pgmodel.Node) (domainnode.TagState, error) {
	q := r.query(ctx)
	rows, err := q.NodeTagRel.WithContext(ctx).Where(q.NodeTagRel.NodeID.Eq(n.ID), q.NodeTagRel.LibraryID.Eq(n.LibraryID)).
		Order(q.NodeTagRel.TagID).Find()
	if err != nil {
		return domainnode.TagState{}, mapDBError(err)
	}
	ids := make([]uint64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, toDomainUint64(row.TagID))
	}
	parents := []uint64{}
	if n.ParentID != nil {
		parents = append(parents, toDomainUint64(*n.ParentID))
	}
	return domainnode.TagState{NodeID: toDomainUint64(n.ID), LibraryID: toDomainUint64(n.LibraryID),
		ParentID: parentIDValue(n.ParentID), TagIDs: ids, AffectedParentIDs: parents}, nil
}

// ApplyNodeTagDelta 必须在 usecase 事务中调用，只修改明确指定的关系并保留其他 metadata。
func (r *NodeRepository) ApplyNodeTagDelta(ctx context.Context, nodeID, libraryID, ownerID uint64, add, remove []uint64) (domainnode.TagState, error) {
	q := r.query(ctx)
	n, err := q.Node.WithContext(ctx).Where(q.Node.ID.Eq(toPGInt64(nodeID)), q.Node.LibraryID.Eq(toPGInt64(libraryID))).
		Clauses(clause.Locking{Strength: "UPDATE"}).First()
	if err != nil {
		return domainnode.TagState{}, mapDBError(err)
	}
	// 锁住指定标签定义，避免校验后被其他账号修改或停用；不触碰未知关系。
	requested := append(append([]uint64{}, add...), remove...)
	var tags []*pgmodel.Tag
	err = r.dbWithContext(ctx).Unscoped().Where("id IN ? AND (owner_user_id = ? OR owner_user_id IS NULL)", toPGInt64Slice(requested), toPGInt64(ownerID)).
		Order("id").Clauses(clause.Locking{Strength: "SHARE"}).Find(&tags).Error
	if err != nil {
		return domainnode.TagState{}, mapDBError(err)
	}
	if len(tags) != len(requested) {
		return domainnode.TagState{}, ErrInvalidState
	}
	if len(add) > 0 {
		if err := r.ensureBindableTags(ctx, ownerID, nodeID, toPGInt64Slice(add)); err != nil {
			return domainnode.TagState{}, err
		}
	}
	var meta map[string]json.RawMessage
	if err := json.Unmarshal([]byte(n.ViewMeta), &meta); err != nil || meta == nil {
		return domainnode.TagState{}, ErrInvalidState
	}
	if len(remove) > 0 {
		_, err = q.NodeTagRel.WithContext(ctx).Where(q.NodeTagRel.NodeID.Eq(n.ID), q.NodeTagRel.LibraryID.Eq(n.LibraryID),
			q.NodeTagRel.TagID.In(toPGInt64Slice(remove)...)).Delete()
		if err != nil {
			return domainnode.TagState{}, mapDBError(err)
		}
	}
	if len(add) > 0 {
		rows := make([]*pgmodel.NodeTagRel, 0, len(add))
		for _, id := range add {
			rows = append(rows, &pgmodel.NodeTagRel{NodeID: n.ID, LibraryID: n.LibraryID, TagID: toPGInt64(id)})
		}
		err = q.NodeTagRel.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "node_id"}, {Name: "tag_id"}}, DoNothing: true}).
			Create(rows...)
		if err != nil {
			return domainnode.TagState{}, mapDBError(err)
		}
	}
	state, err := r.nodeTagState(ctx, n)
	if err != nil {
		return domainnode.TagState{}, err
	}
	meta["tagIds"], err = json.Marshal(state.TagIDs)
	if err != nil {
		return domainnode.TagState{}, err
	}
	encoded, err := json.Marshal(meta)
	if err != nil {
		return domainnode.TagState{}, err
	}
	updated, err := r.UpdateNodeFields(ctx, nodeID, libraryID, map[string]any{"view_meta": string(encoded), "updated_at": time.Now().UTC()})
	if err == nil && !updated {
		return domainnode.TagState{}, ErrNotFound
	}
	return state, err
}
