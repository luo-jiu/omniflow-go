package repository

import (
	"context"
	"strings"

	pgmodel "omniflow-go/internal/repository/postgres/model"

	"github.com/samber/lo"
)

type ArchiveUnitRow struct {
	ID            uint64
	Name          string
	SortOrder     int
	CardKind      string
	ViewMeta      string
	MediaViewMeta string
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

type archiveUnitRawRow struct {
	ID            int64  `gorm:"column:id"`
	Name          string `gorm:"column:name"`
	SortOrder     int    `gorm:"column:sort_order"`
	CardKind      string `gorm:"column:card_kind"`
	ViewMeta      string `gorm:"column:view_meta"`
	MediaViewMeta string `gorm:"column:media_view_meta"`
	MediaNodeID   int64  `gorm:"column:media_node_id"`
	CoverNodeID   int64  `gorm:"column:cover_node_id"`
	SubtitleCount int    `gorm:"column:subtitle_count"`
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

const (
	archiveCardKindMedia      = "media"
	archiveCardKindCollection = "collection"
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
	if normalizedType == "COMIC" {
		return r.listComicArchiveUnits(ctx, parentNodeID, libraryID, offset, limit)
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
			CardKind:  archiveCardKindMedia,
			ViewMeta:  row.ViewMeta,
		}
	})
	return result, int(totalCount), nil
}

func (r *NodeRepository) listDirectChildComicArchiveUnits(
	ctx context.Context,
	parentNodeID uint64,
	libraryID uint64,
	offset int,
	limit int,
) ([]ArchiveUnitRow, int, error) {
	q := r.query(ctx)
	base := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ParentID.Eq(toPGInt64(parentNodeID)),
			q.Node.NodeType.Eq(nodeTypeDirectory),
			q.Node.BuiltInType.Eq("COMIC"),
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
		unit := archiveUnitFromNode(row)
		if row.ArchiveMode {
			unit.CardKind = archiveCardKindCollection
		}
		result = append(result, unit)
	}
	return result, int(totalCount), nil
}

func (r *NodeRepository) listComicArchiveUnits(
	ctx context.Context,
	parentNodeID uint64,
	libraryID uint64,
	offset int,
	limit int,
) ([]ArchiveUnitRow, int, error) {
	return r.listDirectChildComicArchiveUnits(ctx, parentNodeID, libraryID, offset, limit)
}

func archiveUnitFromNode(row *pgmodel.Node) ArchiveUnitRow {
	if row == nil {
		return ArchiveUnitRow{}
	}
	return ArchiveUnitRow{
		ID:        toDomainUint64(row.ID),
		Name:      row.Name,
		SortOrder: int(row.SortOrder),
		CardKind:  archiveCardKindMedia,
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

func archiveMediaKindExtensions(kind archiveMediaKind) []string {
	switch kind {
	case archiveMediaKindVideo:
		return archiveVideoExtensions
	case archiveMediaKindAudio:
		return archiveAudioExtensions
	case archiveMediaKindImage:
		return archiveImageExtensions
	default:
		return []string{}
	}
}

func archiveMediaKindMimePattern(kind archiveMediaKind) string {
	return string(kind) + "/%"
}

func scanArchiveUnitRows(rows []archiveUnitRawRow) []ArchiveUnitRow {
	result := make([]ArchiveUnitRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, ArchiveUnitRow{
			ID:            toDomainUint64(row.ID),
			Name:          row.Name,
			SortOrder:     row.SortOrder,
			CardKind:      strings.TrimSpace(row.CardKind),
			ViewMeta:      row.ViewMeta,
			MediaViewMeta: row.MediaViewMeta,
			MediaNodeID:   toDomainUint64(row.MediaNodeID),
			CoverNodeID:   toDomainUint64(row.CoverNodeID),
			SubtitleCount: row.SubtitleCount,
		})
	}
	return result
}

func archivePagedMediaUnitCandidatesSQL(includeCollections bool) string {
	collectionUnion := ""
	if includeCollections {
		collectionUnion = `
  union all
  select
    d.id,
    d.library_id,
    d.name,
    d.sort_order,
    'collection'::text as card_kind,
    'collection'::text as unit_kind,
    d.view_meta
  from nodes d
  where
    d.deleted_at is null
    and d.library_id = ?
    and d.parent_id = ?
    and d.node_type = ?
    and d.archive_mode = true
    and d.built_in_type = ?`
	}
	return `
with unit_candidates as (
  select
    n.id,
    n.library_id,
    n.name,
    n.sort_order,
    'media'::text as card_kind,
    'direct_file'::text as unit_kind,
    n.view_meta
  from nodes n
  left join node_files nf
    on nf.library_id = n.library_id and nf.file_id = n.id
  where
    n.deleted_at is null
    and n.library_id = ?
    and n.parent_id = ?
    and n.node_type = ?
    and n.name not like '.%'
    and not (n.name = '' and lower(trim(leading '.' from coalesce(n.ext, ''))) <> '')
    and (
      lower(coalesce(nf.mime_type, '')) like ?
      or lower(trim(leading '.' from coalesce(n.ext, ''))) in ?
    )
  union all
  select
    d.id,
    d.library_id,
    d.name,
    d.sort_order,
    'media'::text as card_kind,
    'media_directory'::text as unit_kind,
    d.view_meta
  from nodes d
  where
    d.deleted_at is null
    and d.library_id = ?
    and d.parent_id = ?
    and d.node_type = ?
    and d.archive_mode = false
    and d.built_in_type = ?
    and exists (
      select 1
      from nodes c
      left join node_files cnf
        on cnf.library_id = c.library_id and cnf.file_id = c.id
      where
        c.deleted_at is null
        and c.library_id = d.library_id
        and c.parent_id = d.id
        and c.node_type = ?
        and c.name not like '.%'
        and not (c.name = '' and lower(trim(leading '.' from coalesce(c.ext, ''))) <> '')
        and (
          lower(coalesce(cnf.mime_type, '')) like ?
          or lower(trim(leading '.' from coalesce(c.ext, ''))) in ?
        )
    )` + collectionUnion + `
)`
}

func archivePagedMediaUnitPageSQL(includeCollections bool) string {
	return archivePagedMediaUnitCandidatesSQL(includeCollections) + `,
paged_units as (
  select *
  from unit_candidates
  order by sort_order asc, id asc
  offset ? limit ?
)
select
  pu.id,
  pu.name,
  pu.sort_order,
  pu.card_kind,
  pu.view_meta::text as view_meta,
  case
    when pu.unit_kind = 'direct_file' then pu.view_meta::text
    else coalesce(media.view_meta::text, '')
  end as media_view_meta,
  case
    when pu.unit_kind = 'direct_file' then pu.id
    else coalesce(media.id, 0)::bigint
  end as media_node_id,
  coalesce(cover.id, 0)::bigint as cover_node_id,
  case
    when pu.unit_kind = 'media_directory' then coalesce(subtitle.subtitle_count, 0)::int
    else 0
  end as subtitle_count
from paged_units pu
left join lateral (
    select c.id, c.view_meta
    from nodes c
    left join node_files cnf
      on cnf.library_id = c.library_id and cnf.file_id = c.id
    where
      c.deleted_at is null
      and c.library_id = pu.library_id
      and c.parent_id = pu.id
      and c.node_type = ?
      and c.name not like '.%'
      and not (c.name = '' and lower(trim(leading '.' from coalesce(c.ext, ''))) <> '')
      and (
        lower(coalesce(cnf.mime_type, '')) like ?
        or lower(trim(leading '.' from coalesce(c.ext, ''))) in ?
      )
    order by c.sort_order asc, c.id asc
    limit 1
  ) media on pu.unit_kind = 'media_directory'
left join lateral (
    select c.id
    from nodes c
    left join node_files cnf
      on cnf.library_id = c.library_id and cnf.file_id = c.id
    where
      c.deleted_at is null
      and c.library_id = pu.library_id
      and c.parent_id = pu.id
      and c.node_type = ?
      and c.name not like '.%'
      and not (c.name = '' and lower(trim(leading '.' from coalesce(c.ext, ''))) <> '')
      and (
        lower(coalesce(cnf.mime_type, '')) like ?
        or lower(trim(leading '.' from coalesce(c.ext, ''))) in ?
      )
    order by c.sort_order asc, c.id asc
    limit 1
  ) cover on pu.unit_kind = 'media_directory'
left join lateral (
    select count(*) as subtitle_count
    from nodes c
    where
      c.deleted_at is null
      and c.library_id = pu.library_id
      and c.parent_id = pu.id
      and c.node_type = ?
      and c.name not like '.%'
      and not (c.name = '' and lower(trim(leading '.' from coalesce(c.ext, ''))) <> '')
      and lower(trim(leading '.' from coalesce(c.ext, ''))) in ?
  ) subtitle on pu.unit_kind = 'media_directory'
order by pu.sort_order asc, pu.id asc`
}

func archivePagedMediaUnitCandidateArgs(
	parentNodeID uint64,
	libraryID uint64,
	builtInType string,
	mediaKind archiveMediaKind,
	includeCollections bool,
) []any {
	mediaExtensions := archiveMediaKindExtensions(mediaKind)
	mediaMimePattern := archiveMediaKindMimePattern(mediaKind)
	args := []any{
		toPGInt64(libraryID),
		toPGInt64(parentNodeID),
		nodeTypeFile,
		mediaMimePattern,
		mediaExtensions,
		toPGInt64(libraryID),
		toPGInt64(parentNodeID),
		nodeTypeDirectory,
		builtInType,
		nodeTypeFile,
		mediaMimePattern,
		mediaExtensions,
	}
	if includeCollections {
		args = append(
			args,
			toPGInt64(libraryID),
			toPGInt64(parentNodeID),
			nodeTypeDirectory,
			builtInType,
		)
	}
	return args
}

func archivePagedMediaUnitDetailArgs(mediaKind archiveMediaKind) []any {
	mediaExtensions := archiveMediaKindExtensions(mediaKind)
	mediaMimePattern := archiveMediaKindMimePattern(mediaKind)
	imageMimePattern := archiveMediaKindMimePattern(archiveMediaKindImage)
	return []any{
		nodeTypeFile,
		mediaMimePattern,
		mediaExtensions,
		nodeTypeFile,
		imageMimePattern,
		archiveImageExtensions,
		nodeTypeFile,
		archiveSubtitleExtensions,
	}
}

func (r *NodeRepository) listPagedMediaArchiveUnits(
	ctx context.Context,
	parentNodeID uint64,
	libraryID uint64,
	builtInType string,
	mediaKind archiveMediaKind,
	includeCollections bool,
	offset int,
	limit int,
) ([]ArchiveUnitRow, int, error) {
	candidateSQL := archivePagedMediaUnitCandidatesSQL(includeCollections)
	candidateArgs := archivePagedMediaUnitCandidateArgs(
		parentNodeID,
		libraryID,
		builtInType,
		mediaKind,
		includeCollections,
	)

	var totalCount int64
	if err := r.dbWithContext(ctx).
		Raw(candidateSQL+" select count(*) from unit_candidates", candidateArgs...).
		Scan(&totalCount).Error; err != nil {
		return nil, 0, err
	}
	if totalCount == 0 {
		return []ArchiveUnitRow{}, 0, nil
	}

	pageArgs := append(append([]any{}, candidateArgs...), offset, limit)
	pageArgs = append(pageArgs, archivePagedMediaUnitDetailArgs(mediaKind)...)
	var rows []archiveUnitRawRow
	if err := r.dbWithContext(ctx).
		Raw(archivePagedMediaUnitPageSQL(includeCollections), pageArgs...).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return scanArchiveUnitRows(rows), int(totalCount), nil
}

func (r *NodeRepository) listVideoArchiveUnits(
	ctx context.Context,
	parentNodeID uint64,
	libraryID uint64,
	offset int,
	limit int,
) ([]ArchiveUnitRow, int, error) {
	return r.listPagedMediaArchiveUnits(
		ctx,
		parentNodeID,
		libraryID,
		"VIDEO",
		archiveMediaKindVideo,
		true,
		offset,
		limit,
	)
}

func (r *NodeRepository) listAudioArchiveUnits(
	ctx context.Context,
	parentNodeID uint64,
	libraryID uint64,
	offset int,
	limit int,
) ([]ArchiveUnitRow, int, error) {
	return r.listPagedMediaArchiveUnits(
		ctx,
		parentNodeID,
		libraryID,
		"AUDIO",
		archiveMediaKindAudio,
		false,
		offset,
		limit,
	)
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
			CardKind:  archiveCardKindMedia,
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
		CardKind:  archiveCardKindMedia,
		ViewMeta:  row.ViewMeta,
	}, nil
}
