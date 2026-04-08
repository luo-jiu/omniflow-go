package repository

import (
	"context"
)

const (
	sqlListTreeDescendantRefs = `
WITH RECURSIVE tree AS (
    SELECT id, 0 AS depth
    FROM nodes
    WHERE id = ? AND library_id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT n.id, tree.depth + 1
    FROM nodes n
    JOIN tree ON n.parent_id = tree.id
    WHERE n.library_id = ? AND n.deleted_at IS NULL
)
SELECT id, depth
FROM tree`

	sqlListAncestorPath = `
WITH RECURSIVE ancestors AS (
    SELECT id, name, parent_id, 0 AS depth
    FROM nodes
    WHERE id = ? AND library_id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT p.id, p.name, p.parent_id, ancestors.depth + 1
    FROM nodes p
    JOIN ancestors ON ancestors.parent_id = p.id
    WHERE p.library_id = ? AND p.deleted_at IS NULL
)
SELECT id, name, depth
FROM ancestors
ORDER BY depth DESC`

	sqlListSubtreeNodeIDs = `
WITH RECURSIVE sub AS (
    SELECT id
    FROM nodes
    WHERE id = ? AND library_id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT n.id
    FROM nodes n
    JOIN sub s ON n.parent_id = s.id
    WHERE n.library_id = ? AND n.deleted_at IS NULL
)
SELECT id FROM sub`

	sqlCountSubtreeTargetNode = `
WITH RECURSIVE sub AS (
    SELECT id
    FROM nodes
    WHERE id = ? AND library_id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT n.id
    FROM nodes n
    JOIN sub s ON n.parent_id = s.id
    WHERE n.library_id = ? AND n.deleted_at IS NULL
)
SELECT COUNT(1)
FROM sub
WHERE id = ?`

	sqlListSubtreeNodeIDsAny = `
WITH RECURSIVE sub AS (
    SELECT id
    FROM nodes
    WHERE id = ? AND library_id = ?
    UNION ALL
    SELECT n.id
    FROM nodes n
    JOIN sub s ON n.parent_id = s.id
    WHERE n.library_id = ?
)
SELECT id FROM sub`
)

// scanRaw 统一执行 Raw SQL 查询，避免方法体里重复样板代码。
func (r *NodeRepository) scanRaw(ctx context.Context, dest any, query string, args ...any) error {
	return r.dbWithContext(ctx).Raw(query, args...).Scan(dest).Error
}
