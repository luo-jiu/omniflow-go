package repository

const sqlMetadataSubtreeIDs = `
WITH RECURSIVE subtree AS (
 SELECT id, ARRAY[id] AS visited FROM nodes WHERE id = ? AND library_id = ? AND deleted_at IS NULL
 UNION ALL
 SELECT n.id, s.visited || n.id FROM nodes n JOIN subtree s ON n.parent_id = s.id
 WHERE n.library_id = ? AND n.deleted_at IS NULL AND NOT n.id = ANY(s.visited)
) SELECT id FROM subtree`

const sqlMetadataPaths = `
WITH RECURSIVE paths AS (
 SELECT id AS leaf_id, id, parent_id, name, 0 AS depth, ARRAY[id] AS visited
 FROM nodes WHERE library_id = ? AND id IN ? AND deleted_at IS NULL
 UNION ALL
 SELECT p.leaf_id, n.id, n.parent_id, n.name, p.depth + 1, p.visited || n.id
 FROM nodes n JOIN paths p ON n.id = p.parent_id
 WHERE n.library_id = ? AND n.deleted_at IS NULL AND NOT n.id = ANY(p.visited)
)
SELECT leaf_id AS id,
 '/' || COALESCE(string_agg(name, '/' ORDER BY depth DESC) FILTER (WHERE id <> ?), '') AS path,
 bool_or(id = ?) AS rooted
FROM paths GROUP BY leaf_id`
