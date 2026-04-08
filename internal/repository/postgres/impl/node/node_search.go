package repository

import (
	"context"
	"fmt"
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

// SearchNodes 按关键字与 tagIds 组合搜索节点。
func (r *NodeRepository) SearchNodes(ctx context.Context, input SearchNodesInput) ([]domainnode.Node, error) {
	query := r.dbWithContext(ctx).
		Model(&pgmodel.Node{}).
		Where("library_id = ? AND deleted_at IS NULL", toPGInt64(input.LibraryID))

	if input.Keyword != "" {
		query = query.Where("name LIKE ?", "%"+input.Keyword+"%")
	}

	if len(input.TagIDs) > 0 {
		switch strings.ToUpper(strings.TrimSpace(input.TagMatchMode)) {
		case "ALL":
			for _, tagID := range input.TagIDs {
				query = query.Where(
					"COALESCE(view_meta->'tagIds', '[]'::jsonb) @> ?::jsonb",
					fmt.Sprintf("[%d]", tagID),
				)
			}
		default:
			conditions := make([]string, 0, len(input.TagIDs))
			args := make([]any, 0, len(input.TagIDs))
			for _, tagID := range input.TagIDs {
				conditions = append(conditions, "COALESCE(view_meta->'tagIds', '[]'::jsonb) @> ?::jsonb")
				args = append(args, fmt.Sprintf("[%d]", tagID))
			}
			query = query.Where("("+strings.Join(conditions, " OR ")+")", args...)
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
