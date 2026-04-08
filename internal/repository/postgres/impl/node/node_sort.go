package repository

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode"

	pgmodel "omniflow-go/internal/repository/postgres/model"
)

const comicSortStep = 1024

// SortDirectChildrenByName 按自然名称顺序重排目录直接子节点。
func (r *NodeRepository) SortDirectChildrenByName(ctx context.Context, nodeID, libraryID uint64, updatedAt time.Time) error {
	parent, err := r.findNodeModel(ctx, nodeID, libraryID)
	if err != nil {
		return err
	}
	if parent.NodeType != nodeTypeDirectory {
		return ErrInvalidState
	}

	q := r.query(ctx)
	rows, err := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ParentID.Eq(toPGInt64(nodeID)),
		).
		Order(q.Node.ID.Asc()).
		Find()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	sort.SliceStable(rows, func(i, j int) bool {
		leftName := sortableNodeName(rows[i])
		rightName := sortableNodeName(rows[j])
		cmp := naturalCompare(leftName, rightName)
		if cmp != 0 {
			return cmp < 0
		}
		return rows[i].ID < rows[j].ID
	})

	nextOrder := comicSortStep
	for _, row := range rows {
		info, err := q.Node.WithContext(ctx).
			Where(
				q.Node.ID.Eq(row.ID),
				q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			).
			Updates(map[string]any{
				"sort_order": nextOrder,
				"updated_at": updatedAt,
			})
		if err != nil {
			return err
		}
		if info.RowsAffected == 0 {
			return ErrNotFound
		}
		nextOrder += comicSortStep
	}
	return nil
}

func sortableNodeName(node *pgmodel.Node) string {
	baseName := strings.TrimSpace(node.Name)
	if node.NodeType != nodeTypeFile {
		return baseName
	}

	ext := strings.TrimSpace(derefString(node.Ext))
	if ext == "" {
		return baseName
	}
	return baseName + "." + strings.ToLower(ext)
}

func naturalCompare(left, right string) int {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	li := 0
	ri := 0

	for li < len(leftRunes) && ri < len(rightRunes) {
		lch := leftRunes[li]
		rch := rightRunes[ri]
		leftIsDigit := unicode.IsDigit(lch)
		rightIsDigit := unicode.IsDigit(rch)

		if leftIsDigit && rightIsDigit {
			leftNumStart := li
			rightNumStart := ri
			for li < len(leftRunes) && unicode.IsDigit(leftRunes[li]) {
				li++
			}
			for ri < len(rightRunes) && unicode.IsDigit(rightRunes[ri]) {
				ri++
			}

			leftNoZero := leftNumStart
			for leftNoZero < li && leftRunes[leftNoZero] == '0' {
				leftNoZero++
			}
			rightNoZero := rightNumStart
			for rightNoZero < ri && rightRunes[rightNoZero] == '0' {
				rightNoZero++
			}

			leftNumericLength := li - leftNoZero
			rightNumericLength := ri - rightNoZero
			if leftNumericLength != rightNumericLength {
				if leftNumericLength < rightNumericLength {
					return -1
				}
				return 1
			}

			for i := 0; i < leftNumericLength; i++ {
				leftValue := leftRunes[leftNoZero+i]
				rightValue := rightRunes[rightNoZero+i]
				if leftValue != rightValue {
					if leftValue < rightValue {
						return -1
					}
					return 1
				}
			}

			leftRawLength := li - leftNumStart
			rightRawLength := ri - rightNumStart
			if leftRawLength != rightRawLength {
				if leftRawLength < rightRawLength {
					return -1
				}
				return 1
			}
			continue
		}

		leftLower := unicode.ToLower(lch)
		rightLower := unicode.ToLower(rch)
		if leftLower != rightLower {
			if leftLower < rightLower {
				return -1
			}
			return 1
		}
		li++
		ri++
	}

	if len(leftRunes) < len(rightRunes) {
		return -1
	}
	if len(leftRunes) > len(rightRunes) {
		return 1
	}
	return 0
}
