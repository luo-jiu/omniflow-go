package repository

import (
	"context"
	"sort"
	"strings"

	pgmodel "omniflow-go/internal/repository/postgres/model"

	"github.com/samber/lo"
)

type ArchiveUnitRow struct {
	ID            uint64
	Name          string
	SortOrder     int
	ViewMeta      string
	MediaNodeID   uint64
	CoverNodeID   uint64
	SubtitleCount int
}

// StorageInfoRow 文件节点的存储位置信息。
type StorageInfoRow struct {
	NodeID        int64  `gorm:"column:node_id"`
	StorageKey    string `gorm:"column:storage_key"`
	ProviderAlias string `gorm:"column:provider_alias"`
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

var archiveVideoExtensions = []string{
	"mp4",
	"m4v",
	"webm",
	"mkv",
	"mov",
	"avi",
	"ts",
	"flv",
	"hlv",
	"f4v",
	"mpeg",
	"mpg",
	"wmv",
	"asf",
	"movie",
	"divx",
	"mpeg4",
	"vid",
	"ogv",
	"3gp",
}

var archiveAudioExtensions = []string{
	"mp3",
	"wav",
	"aac",
	"flac",
	"m4a",
	"ogg",
	"oga",
	"opus",
}

var archiveSubtitleExtensions = []string{
	"lrc",
	"srt",
	"vtt",
	"ass",
	"ssa",
}

type archiveMediaKind string

const (
	archiveMediaKindImage archiveMediaKind = "image"
	archiveMediaKindVideo archiveMediaKind = "video"
	archiveMediaKindAudio archiveMediaKind = "audio"
)

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
	if normalizedType == "VIDEO" {
		return r.listVideoArchiveUnits(ctx, parentNodeID, libraryID, offset, limit)
	}
	if normalizedType == "AUDIO" {
		return r.listAudioArchiveUnits(ctx, parentNodeID, libraryID, offset, limit)
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
	result = lo.Map(rows, func(row *pgmodel.Node, _ int) ArchiveUnitRow {
		return ArchiveUnitRow{
			ID:        toDomainUint64(row.ID),
			Name:      row.Name,
			SortOrder: int(row.SortOrder),
			ViewMeta:  row.ViewMeta,
		}
	})
	return result, int(totalCount), nil
}

func archiveUnitFromNode(row *pgmodel.Node) ArchiveUnitRow {
	if row == nil {
		return ArchiveUnitRow{}
	}
	return ArchiveUnitRow{
		ID:        toDomainUint64(row.ID),
		Name:      row.Name,
		SortOrder: int(row.SortOrder),
		ViewMeta:  row.ViewMeta,
	}
}

func normalizeArchiveNodeExt(row *pgmodel.Node) string {
	if row == nil || row.Ext == nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(*row.Ext)), ".")
}

func isVisibleArchiveFileNode(row *pgmodel.Node) bool {
	if row == nil || row.NodeType != nodeTypeFile {
		return false
	}
	name := strings.TrimSpace(row.Name)
	if strings.HasPrefix(name, ".") {
		return false
	}
	return !(name == "" && normalizeArchiveNodeExt(row) != "")
}

func archiveExtMatches(kind archiveMediaKind, ext string) bool {
	switch kind {
	case archiveMediaKindImage:
		return lo.Contains(archiveImageExtensions, ext)
	case archiveMediaKindVideo:
		return lo.Contains(archiveVideoExtensions, ext)
	case archiveMediaKindAudio:
		return lo.Contains(archiveAudioExtensions, ext)
	default:
		return false
	}
}

func archiveMimeMatches(kind archiveMediaKind, mimeType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(mimeType))
	if normalized == "" {
		return false
	}
	return strings.HasPrefix(normalized, string(kind)+"/")
}

func isArchiveMediaNode(row *pgmodel.Node, mimeTypes map[uint64]string, kind archiveMediaKind) bool {
	if !isVisibleArchiveFileNode(row) {
		return false
	}
	nodeID := toDomainUint64(row.ID)
	if archiveMimeMatches(kind, mimeTypes[nodeID]) {
		return true
	}
	return archiveExtMatches(kind, normalizeArchiveNodeExt(row))
}

func isArchiveSubtitleNode(row *pgmodel.Node) bool {
	if !isVisibleArchiveFileNode(row) {
		return false
	}
	return lo.Contains(archiveSubtitleExtensions, normalizeArchiveNodeExt(row))
}

func sortArchiveUnits(units []ArchiveUnitRow) {
	sort.SliceStable(units, func(i, j int) bool {
		if units[i].SortOrder != units[j].SortOrder {
			return units[i].SortOrder < units[j].SortOrder
		}
		return units[i].ID < units[j].ID
	})
}

func paginateArchiveUnits(units []ArchiveUnitRow, offset int, limit int) []ArchiveUnitRow {
	if offset >= len(units) {
		return []ArchiveUnitRow{}
	}
	end := offset + limit
	if end > len(units) {
		end = len(units)
	}
	return units[offset:end]
}

func collectNodeIDs(rows []*pgmodel.Node) []uint64 {
	result := make([]uint64, 0, len(rows))
	for _, row := range rows {
		nodeID := toDomainUint64(row.ID)
		if nodeID > 0 {
			result = append(result, nodeID)
		}
	}
	return result
}

func (r *NodeRepository) listMimeTypesByNodeIDs(
	ctx context.Context,
	libraryID uint64,
	nodeIDs []uint64,
) (map[uint64]string, error) {
	if len(nodeIDs) == 0 {
		return map[uint64]string{}, nil
	}

	q := r.query(ctx)
	rows, err := q.NodeFile.WithContext(ctx).
		Select(q.NodeFile.FileID, q.NodeFile.MimeType).
		Where(
			q.NodeFile.LibraryID.Eq(toPGInt64(libraryID)),
			q.NodeFile.FileID.In(toPGInt64Slice(nodeIDs)...),
		).
		Find()
	if err != nil {
		return nil, err
	}

	result := make(map[uint64]string, len(rows))
	for _, row := range rows {
		nodeID := toDomainUint64(row.FileID)
		if nodeID == 0 {
			continue
		}
		result[nodeID] = strings.TrimSpace(derefString(row.MimeType))
	}
	return result, nil
}

func (r *NodeRepository) listLiveFileNodeIDsByIDs(
	ctx context.Context,
	libraryID uint64,
	nodeIDs []uint64,
) ([]uint64, error) {
	if len(nodeIDs) == 0 {
		return []uint64{}, nil
	}

	q := r.query(ctx)
	rows, err := q.Node.WithContext(ctx).
		Select(q.Node.ID).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ID.In(toPGInt64Slice(nodeIDs)...),
			q.Node.NodeType.Eq(nodeTypeFile),
		).
		Find()
	if err != nil {
		return nil, err
	}
	return collectNodeIDs(rows), nil
}

func (r *NodeRepository) listDirectChildFileArchiveUnits(
	ctx context.Context,
	parentNodeID uint64,
	libraryID uint64,
	kind archiveMediaKind,
) ([]ArchiveUnitRow, error) {
	q := r.query(ctx)
	rows, err := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ParentID.Eq(toPGInt64(parentNodeID)),
			q.Node.NodeType.Eq(nodeTypeFile),
		).
		Order(
			q.Node.SortOrder.Asc(),
			q.Node.ID.Asc(),
		).
		Find()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []ArchiveUnitRow{}, nil
	}

	mimeTypes, err := r.listMimeTypesByNodeIDs(ctx, libraryID, collectNodeIDs(rows))
	if err != nil {
		return nil, err
	}

	units := make([]ArchiveUnitRow, 0, len(rows))
	for _, row := range rows {
		if !isArchiveMediaNode(row, mimeTypes, kind) {
			continue
		}
		unit := archiveUnitFromNode(row)
		unit.MediaNodeID = unit.ID
		units = append(units, unit)
	}
	return units, nil
}

func (r *NodeRepository) listDirectChildVideoDirectoryArchiveUnits(
	ctx context.Context,
	parentNodeID uint64,
	libraryID uint64,
) ([]ArchiveUnitRow, error) {
	q := r.query(ctx)
	directories, err := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ParentID.Eq(toPGInt64(parentNodeID)),
			q.Node.NodeType.Eq(nodeTypeDirectory),
			q.Node.ArchiveMode.Is(false),
			q.Node.BuiltInType.Eq("VIDEO"),
		).
		Order(
			q.Node.SortOrder.Asc(),
			q.Node.ID.Asc(),
		).
		Find()
	if err != nil {
		return nil, err
	}
	if len(directories) == 0 {
		return []ArchiveUnitRow{}, nil
	}

	directoryIDs := collectNodeIDs(directories)
	children, err := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ParentID.In(toPGInt64Slice(directoryIDs)...),
			q.Node.NodeType.Eq(nodeTypeFile),
		).
		Order(
			q.Node.ParentID.Asc(),
			q.Node.SortOrder.Asc(),
			q.Node.ID.Asc(),
		).
		Find()
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return []ArchiveUnitRow{}, nil
	}

	mimeTypes, err := r.listMimeTypesByNodeIDs(ctx, libraryID, collectNodeIDs(children))
	if err != nil {
		return nil, err
	}

	unitByDirectoryID := make(map[uint64]ArchiveUnitRow, len(directories))
	for _, directory := range directories {
		unit := archiveUnitFromNode(directory)
		unitByDirectoryID[unit.ID] = unit
	}

	for _, child := range children {
		if child.ParentID == nil {
			continue
		}
		parentID := toDomainUint64(*child.ParentID)
		unit, ok := unitByDirectoryID[parentID]
		if !ok {
			continue
		}
		if unit.MediaNodeID == 0 && isArchiveMediaNode(child, mimeTypes, archiveMediaKindVideo) {
			unit.MediaNodeID = toDomainUint64(child.ID)
		}
		if unit.CoverNodeID == 0 && isArchiveMediaNode(child, mimeTypes, archiveMediaKindImage) {
			unit.CoverNodeID = toDomainUint64(child.ID)
		}
		if isArchiveSubtitleNode(child) {
			unit.SubtitleCount++
		}
		unitByDirectoryID[parentID] = unit
	}

	units := make([]ArchiveUnitRow, 0, len(directories))
	for _, directory := range directories {
		nodeID := toDomainUint64(directory.ID)
		unit := unitByDirectoryID[nodeID]
		if unit.MediaNodeID == 0 {
			continue
		}
		units = append(units, unit)
	}
	return units, nil
}

func (r *NodeRepository) listVideoArchiveUnits(
	ctx context.Context,
	parentNodeID uint64,
	libraryID uint64,
	offset int,
	limit int,
) ([]ArchiveUnitRow, int, error) {
	fileUnits, err := r.listDirectChildFileArchiveUnits(ctx, parentNodeID, libraryID, archiveMediaKindVideo)
	if err != nil {
		return nil, 0, err
	}
	directoryUnits, err := r.listDirectChildVideoDirectoryArchiveUnits(ctx, parentNodeID, libraryID)
	if err != nil {
		return nil, 0, err
	}

	units := append(fileUnits, directoryUnits...)
	sortArchiveUnits(units)
	total := len(units)
	if total == 0 {
		return []ArchiveUnitRow{}, 0, nil
	}
	return paginateArchiveUnits(units, offset, limit), total, nil
}

func (r *NodeRepository) listAudioArchiveUnits(
	ctx context.Context,
	parentNodeID uint64,
	libraryID uint64,
	offset int,
	limit int,
) ([]ArchiveUnitRow, int, error) {
	units, err := r.listDirectChildFileArchiveUnits(ctx, parentNodeID, libraryID, archiveMediaKindAudio)
	if err != nil {
		return nil, 0, err
	}
	total := len(units)
	if total == 0 {
		return []ArchiveUnitRow{}, 0, nil
	}
	return paginateArchiveUnits(units, offset, limit), total, nil
}

func (r *NodeRepository) DetectFirstImageChildrenByParentIDs(
	ctx context.Context,
	libraryID uint64,
	parentNodeIDs []uint64,
) (map[uint64]uint64, error) {
	if len(parentNodeIDs) == 0 {
		return map[uint64]uint64{}, nil
	}

	q := r.query(ctx)
	rows, err := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ParentID.In(toPGInt64Slice(parentNodeIDs)...),
			q.Node.NodeType.Eq(nodeTypeFile),
		).
		Order(
			q.Node.ParentID.Asc(),
			q.Node.SortOrder.Asc(),
			q.Node.ID.Asc(),
		).
		Find()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return map[uint64]uint64{}, nil
	}
	mimeTypes, err := r.listMimeTypesByNodeIDs(ctx, libraryID, collectNodeIDs(rows))
	if err != nil {
		return nil, err
	}

	result := make(map[uint64]uint64, len(parentNodeIDs))
	for _, row := range rows {
		if row.ParentID == nil || !isArchiveMediaNode(row, mimeTypes, archiveMediaKindImage) {
			continue
		}
		parentID := toDomainUint64(*row.ParentID)
		if parentID == 0 {
			continue
		}
		if _, exists := result[parentID]; exists {
			continue
		}
		result[parentID] = toDomainUint64(row.ID)
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

	liveNodeIDs, err := r.listLiveFileNodeIDsByIDs(ctx, libraryID, nodeIDs)
	if err != nil {
		return nil, err
	}
	if len(liveNodeIDs) == 0 {
		return map[uint64]string{}, nil
	}

	q := r.query(ctx)
	fileRows, err := q.NodeFile.WithContext(ctx).
		Select(q.NodeFile.FileID, q.NodeFile.StorageObjectID).
		Where(
			q.NodeFile.LibraryID.Eq(toPGInt64(libraryID)),
			q.NodeFile.FileID.In(toPGInt64Slice(liveNodeIDs)...),
		).
		Find()
	if err != nil {
		return nil, err
	}
	if len(fileRows) == 0 {
		return map[uint64]string{}, nil
	}

	storageIDs := make([]int64, 0, len(fileRows))
	fileIDsByStorageID := make(map[int64][]uint64, len(fileRows))
	for _, row := range fileRows {
		nodeID := toDomainUint64(row.FileID)
		if nodeID == 0 || row.StorageObjectID <= 0 {
			continue
		}
		storageIDs = append(storageIDs, row.StorageObjectID)
		fileIDsByStorageID[row.StorageObjectID] = append(fileIDsByStorageID[row.StorageObjectID], nodeID)
	}
	if len(storageIDs) == 0 {
		return map[uint64]string{}, nil
	}

	storageRows, err := q.StorageObject.WithContext(ctx).
		Select(q.StorageObject.ID, q.StorageObject.ObjectKey).
		Where(
			q.StorageObject.LibraryID.Eq(toPGInt64(libraryID)),
			q.StorageObject.ID.In(storageIDs...),
			q.StorageObject.ObjectKey.Neq(""),
		).
		Find()
	if err != nil {
		return nil, err
	}

	result := make(map[uint64]string, len(storageRows))
	for _, row := range storageRows {
		storageKey := strings.TrimSpace(row.ObjectKey)
		if storageKey == "" {
			continue
		}
		for _, nodeID := range fileIDsByStorageID[row.ID] {
			if nodeID == 0 {
				continue
			}
			result[nodeID] = storageKey
		}
	}
	return result, nil
}

// ListStorageInfoByNodeIDs 批量查询文件节点的 storageKey + providerAlias。
func (r *NodeRepository) ListStorageInfoByNodeIDs(
	ctx context.Context,
	libraryID uint64,
	nodeIDs []uint64,
) ([]StorageInfoRow, error) {
	if len(nodeIDs) == 0 {
		return []StorageInfoRow{}, nil
	}

	liveNodeIDs, err := r.listLiveFileNodeIDsByIDs(ctx, libraryID, nodeIDs)
	if err != nil {
		return nil, err
	}
	if len(liveNodeIDs) == 0 {
		return []StorageInfoRow{}, nil
	}

	q := r.query(ctx)
	fileRows, err := q.NodeFile.WithContext(ctx).
		Select(q.NodeFile.FileID, q.NodeFile.StorageObjectID).
		Where(
			q.NodeFile.LibraryID.Eq(toPGInt64(libraryID)),
			q.NodeFile.FileID.In(toPGInt64Slice(liveNodeIDs)...),
		).
		Find()
	if err != nil {
		return nil, err
	}
	if len(fileRows) == 0 {
		return []StorageInfoRow{}, nil
	}

	storageIDs := make([]int64, 0, len(fileRows))
	fileIDsByStorageID := make(map[int64][]int64, len(fileRows))
	for _, row := range fileRows {
		if row.FileID <= 0 || row.StorageObjectID <= 0 {
			continue
		}
		storageIDs = append(storageIDs, row.StorageObjectID)
		fileIDsByStorageID[row.StorageObjectID] = append(fileIDsByStorageID[row.StorageObjectID], row.FileID)
	}
	if len(storageIDs) == 0 {
		return []StorageInfoRow{}, nil
	}

	storageRows, err := q.StorageObject.WithContext(ctx).
		Select(q.StorageObject.ID, q.StorageObject.ObjectKey, q.StorageObject.Provider).
		Where(
			q.StorageObject.LibraryID.Eq(toPGInt64(libraryID)),
			q.StorageObject.ID.In(storageIDs...),
			q.StorageObject.ObjectKey.Neq(""),
		).
		Find()
	if err != nil {
		return nil, err
	}

	rows := make([]StorageInfoRow, 0, len(storageRows))
	for _, row := range storageRows {
		storageKey := strings.TrimSpace(row.ObjectKey)
		if storageKey == "" {
			continue
		}
		for _, nodeID := range fileIDsByStorageID[row.ID] {
			if nodeID <= 0 {
				continue
			}
			rows = append(rows, StorageInfoRow{
				NodeID:        nodeID,
				StorageKey:    storageKey,
				ProviderAlias: strings.TrimSpace(row.Provider),
			})
		}
	}
	return rows, nil
}

// GetStorageProviderByNodeID 查询单个文件节点的 provider alias。
func (r *NodeRepository) GetStorageProviderByNodeID(
	ctx context.Context,
	nodeID, libraryID uint64,
) (string, error) {
	rows, err := r.ListStorageInfoByNodeIDs(ctx, libraryID, []uint64{nodeID})
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", ErrNotFound
	}
	return strings.TrimSpace(rows[0].ProviderAlias), nil
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
	result = lo.Map(rows, func(row *pgmodel.Node, _ int) ArchiveUnitRow {
		return ArchiveUnitRow{
			ID:        toDomainUint64(row.ID),
			Name:      row.Name,
			SortOrder: int(row.SortOrder),
			ViewMeta:  row.ViewMeta,
		}
	})
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
