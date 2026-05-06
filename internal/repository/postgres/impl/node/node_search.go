package repository

import (
	"context"
	"sort"
	"strings"

	domainnode "omniflow-go/internal/domain/node"
	pgmodel "omniflow-go/internal/repository/postgres/model"
)

type SearchNodesInput struct {
	LibraryID    uint64
	Keyword      string
	TagIDs       []uint64
	TagMatchMode string
	Limit        int
}

// SearchNodes 按关键字与 node_tag_rel 标签关系组合搜索节点。
func (r *NodeRepository) SearchNodes(ctx context.Context, input SearchNodesInput) ([]domainnode.Node, error) {
	query := r.dbWithContext(ctx).
		Model(&pgmodel.Node{}).
		Where("library_id = ? AND deleted_at IS NULL", toPGInt64(input.LibraryID))

	if input.Keyword != "" {
		query = query.Where("name LIKE ?", "%"+input.Keyword+"%")
	}

	if len(input.TagIDs) > 0 {
		tagIDs := toPGInt64Slice(input.TagIDs)
		switch strings.ToUpper(strings.TrimSpace(input.TagMatchMode)) {
		case "ALL":
			query = query.Where(
				`(
					SELECT COUNT(DISTINCT rel.tag_id)
					FROM node_tag_rel rel
					WHERE rel.library_id = nodes.library_id
					  AND rel.node_id = nodes.id
					  AND rel.tag_id IN ?
				) = ?`,
				tagIDs,
				len(tagIDs),
			)
		default:
			query = query.Where(
				`EXISTS (
					SELECT 1
					FROM node_tag_rel rel
					WHERE rel.library_id = nodes.library_id
					  AND rel.node_id = nodes.id
					  AND rel.tag_id IN ?
				)`,
				tagIDs,
			)
		}
	}

	var rows []*pgmodel.Node
	if err := query.
		Order("updated_at DESC").
		Order("id DESC").
		Limit(input.Limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []domainnode.Node{}, nil
	}

	ids := make([]uint64, 0, len(rows))
	order := make(map[uint64]int, len(rows))
	for i, row := range rows {
		id := toDomainUint64(row.ID)
		ids = append(ids, id)
		order[id] = i
	}

	nodes, err := r.loadNodesWithFileMeta(ctx, input.LibraryID, ids, nil)
	if err != nil {
		return nil, err
	}
	sort.Slice(nodes, func(i, j int) bool {
		return order[nodes[i].Node.ID] < order[nodes[j].Node.ID]
	})

	result := make([]domainnode.Node, 0, len(nodes))
	for _, item := range nodes {
		result = append(result, item.Node)
	}
	return result, nil
}
