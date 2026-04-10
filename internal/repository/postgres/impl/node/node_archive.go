package repository

import (
	"context"
	"strings"
)

type ArchiveUnitRow struct {
	ID        uint64
	Name      string
	SortOrder int
	ViewMeta  string
}

type archiveFirstImageRow struct {
	ParentID int64 `gorm:"column:parent_id"`
	NodeID   int64 `gorm:"column:node_id"`
}

type storageKeyRow struct {
	NodeID     int64  `gorm:"column:node_id"`
	StorageKey string `gorm:"column:storage_key"`
}

var archiveImageExtensions = []string{
	"jpg",
	"jpeg",
	"png",
	"gif",
	"bmp",
	"webp",
	"svg",
	"avif",
	"thumb",
}

const sqlDetectFirstImageChildrenByParentIDs = `
WITH ranked AS (
    SELECT
        n.parent_id,
        n.id AS node_id,
        ROW_NUMBER() OVER (PARTITION BY n.parent_id ORDER BY n.sort_order ASC, n.id ASC) AS rn
    FROM nodes n
    LEFT JOIN node_files nf
        ON nf.file_id = n.id
       AND nf.library_id = n.library_id
    WHERE n.library_id = ?
      AND n.parent_id IN ?
      AND n.node_type = 1
      AND n.deleted_at IS NULL
      AND n.name NOT LIKE '.%%'
      AND NOT (COALESCE(n.name, '') = '' AND COALESCE(n.ext, '') <> '')
      AND (
            LOWER(COALESCE(nf.mime_type, '')) LIKE 'image/%%'
         OR LOWER(COALESCE(n.ext, '')) IN ?
      )
)
SELECT parent_id, node_id
FROM ranked
WHERE rn = 1
`

const sqlListStorageKeysByNodeIDs = `
SELECT
    nf.file_id AS node_id,
    so.object_key AS storage_key
FROM node_files nf
JOIN nodes n
  ON n.id = nf.file_id
 AND n.library_id = nf.library_id
 AND n.deleted_at IS NULL
 AND n.node_type = 1
JOIN storage_objects so
  ON so.id = nf.storage_object_id
 AND so.library_id = nf.library_id
 AND so.deleted_at IS NULL
WHERE nf.library_id = ?
  AND nf.file_id IN ?
`

func (r *NodeRepository) ListArchiveUnitsByBuiltInType(
	ctx context.Context,
	parentNodeID uint64,
	libraryID uint64,
	builtInType string,
	offset int,
	limit int,
) ([]ArchiveUnitRow, int, error) {
	normalizedType := strings.ToUpper(strings.TrimSpace(builtInType))
	if normalizedType == "" {
		return []ArchiveUnitRow{}, 0, nil
	}

	q := r.query(ctx)
	base := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ParentID.Eq(toPGInt64(parentNodeID)),
			q.Node.NodeType.Eq(nodeTypeDirectory),
			q.Node.ArchiveMode.Is(false),
			q.Node.BuiltInType.Eq(normalizedType),
		)

	totalCount, err := base.Count()
	if err != nil {
		return nil, 0, err
	}
	if totalCount == 0 {
		return []ArchiveUnitRow{}, 0, nil
	}

	rows, err := base.
		Order(
			q.Node.SortOrder.Asc(),
			q.Node.ID.Asc(),
		).
		Offset(offset).
		Limit(limit).
		Find()
	if err != nil {
		return nil, 0, err
	}

	result := make([]ArchiveUnitRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, ArchiveUnitRow{
			ID:        toDomainUint64(row.ID),
			Name:      row.Name,
			SortOrder: int(row.SortOrder),
			ViewMeta:  row.ViewMeta,
		})
	}
	return result, int(totalCount), nil
}

func (r *NodeRepository) DetectFirstImageChildrenByParentIDs(
	ctx context.Context,
	libraryID uint64,
	parentNodeIDs []uint64,
) (map[uint64]uint64, error) {
	if len(parentNodeIDs) == 0 {
		return map[uint64]uint64{}, nil
	}

	parentIDs := toPGInt64Slice(parentNodeIDs)
	rows := make([]archiveFirstImageRow, 0, len(parentNodeIDs))
	if err := r.scanRaw(
		ctx,
		&rows,
		sqlDetectFirstImageChildrenByParentIDs,
		toPGInt64(libraryID),
		parentIDs,
		archiveImageExtensions,
	); err != nil {
		return nil, err
	}

	result := make(map[uint64]uint64, len(rows))
	for _, row := range rows {
		parentID := toDomainUint64(row.ParentID)
		nodeID := toDomainUint64(row.NodeID)
		if parentID == 0 || nodeID == 0 {
			continue
		}
		result[parentID] = nodeID
	}
	return result, nil
}

func (r *NodeRepository) ListStorageKeysByNodeIDs(
	ctx context.Context,
	libraryID uint64,
	nodeIDs []uint64,
) (map[uint64]string, error) {
	if len(nodeIDs) == 0 {
		return map[uint64]string{}, nil
	}

	rows := make([]storageKeyRow, 0, len(nodeIDs))
	if err := r.scanRaw(
		ctx,
		&rows,
		sqlListStorageKeysByNodeIDs,
		toPGInt64(libraryID),
		toPGInt64Slice(nodeIDs),
	); err != nil {
		return nil, err
	}

	result := make(map[uint64]string, len(rows))
	for _, row := range rows {
		nodeID := toDomainUint64(row.NodeID)
		if nodeID == 0 || strings.TrimSpace(row.StorageKey) == "" {
			continue
		}
		result[nodeID] = row.StorageKey
	}
	return result, nil
}

func (r *NodeRepository) ListDirectChildDirectoryNodesByBuiltInType(
	ctx context.Context,
	parentNodeID uint64,
	libraryID uint64,
	builtInType string,
) ([]ArchiveUnitRow, error) {
	normalizedType := strings.ToUpper(strings.TrimSpace(builtInType))
	if normalizedType == "" {
		return []ArchiveUnitRow{}, nil
	}

	q := r.query(ctx)
	rows, err := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ParentID.Eq(toPGInt64(parentNodeID)),
			q.Node.NodeType.Eq(nodeTypeDirectory),
			q.Node.BuiltInType.Eq(normalizedType),
		).
		Order(
			q.Node.SortOrder.Asc(),
			q.Node.ID.Asc(),
		).
		Find()
	if err != nil {
		return nil, err
	}

	result := make([]ArchiveUnitRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, ArchiveUnitRow{
			ID:        toDomainUint64(row.ID),
			Name:      row.Name,
			SortOrder: int(row.SortOrder),
			ViewMeta:  row.ViewMeta,
		})
	}
	return result, nil
}

func (r *NodeRepository) FindArchiveUnitByID(
	ctx context.Context,
	nodeID uint64,
	libraryID uint64,
	builtInType string,
) (ArchiveUnitRow, error) {
	normalizedType := strings.ToUpper(strings.TrimSpace(builtInType))
	if normalizedType == "" {
		return ArchiveUnitRow{}, ErrNotFound
	}

	q := r.query(ctx)
	row, err := q.Node.WithContext(ctx).
		Where(
			q.Node.ID.Eq(toPGInt64(nodeID)),
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.NodeType.Eq(nodeTypeDirectory),
			q.Node.BuiltInType.Eq(normalizedType),
		).
		First()
	if err != nil {
		return ArchiveUnitRow{}, mapDBError(err)
	}

	return ArchiveUnitRow{
		ID:        toDomainUint64(row.ID),
		Name:      row.Name,
		SortOrder: int(row.SortOrder),
		ViewMeta:  row.ViewMeta,
	}, nil
}
