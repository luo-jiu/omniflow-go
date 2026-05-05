package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"omniflow-go/internal/actor"
	domainnode "omniflow-go/internal/domain/node"
)

type ListArchiveCardsQuery struct {
	Actor       actor.Actor
	LibraryID   uint64
	NodeID      uint64
	BuiltInType string
	Offset      int
	Limit       int
}

type ArchiveCardItem struct {
	ID              uint64  `json:"id"`
	Name            string  `json:"name"`
	SortOrder       int     `json:"sortOrder"`
	ViewMeta        string  `json:"viewMeta,omitempty"`
	CoverNodeID     uint64  `json:"coverNodeId,omitempty"`
	MediaNodeID     uint64  `json:"mediaNodeId,omitempty"`
	SubtitleCount   int     `json:"subtitleCount,omitempty"`
	DurationSeconds float64 `json:"durationSeconds,omitempty"`
}

type ListArchiveCardsResult struct {
	Items   []ArchiveCardItem `json:"items"`
	Total   int               `json:"total"`
	Offset  int               `json:"offset"`
	Limit   int               `json:"limit"`
	HasMore bool              `json:"hasMore"`
}

func normalizeArchiveCardBuiltInType(input string) string {
	normalized := strings.ToUpper(strings.TrimSpace(input))
	if normalized == "COMIC" || normalized == "ASMR" || normalized == "VIDEO" || normalized == "AUDIO" {
		return normalized
	}
	return ""
}

func normalizeArchiveCardPageLimit(value int) int {
	if value <= 0 {
		return 24
	}
	if value > 120 {
		return 120
	}
	return value
}

func normalizeArchiveCardOffset(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func (u *NodeUseCase) ListArchiveCards(
	ctx context.Context,
	query ListArchiveCardsQuery,
) (ListArchiveCardsResult, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return ListArchiveCardsResult{}, err
	}

	if query.LibraryID == 0 || query.NodeID == 0 {
		return ListArchiveCardsResult{}, fmt.Errorf("%w: library id and node id are required", ErrInvalidArgument)
	}

	builtInType := normalizeArchiveCardBuiltInType(query.BuiltInType)
	if builtInType == "" {
		return ListArchiveCardsResult{}, fmt.Errorf("%w: unsupported built-in type", ErrInvalidArgument)
	}
	if err := u.AuthorizeRead(ctx, query.Actor, query.LibraryID); err != nil {
		return ListArchiveCardsResult{}, err
	}

	parentNode, err := u.findNodeView(ctx, query.NodeID, query.LibraryID)
	if err != nil {
		return ListArchiveCardsResult{}, err
	}
	if parentNode.Type != domainnode.TypeDirectory {
		return ListArchiveCardsResult{}, fmt.Errorf("%w: node must be directory", ErrInvalidArgument)
	}

	offset := normalizeArchiveCardOffset(query.Offset)
	limit := normalizeArchiveCardPageLimit(query.Limit)
	units, total, err := u.nodes.ListArchiveUnitsByBuiltInType(
		ctx,
		query.NodeID,
		query.LibraryID,
		builtInType,
		offset,
		limit,
	)
	if err != nil {
		return ListArchiveCardsResult{}, err
	}
	if len(units) == 0 {
		slog.DebugContext(ctx, "node.archive.cards.listed",
			"node_id", query.NodeID,
			"library_id", query.LibraryID,
			"built_in_type", builtInType,
			"offset", offset,
			"limit", limit,
			"result_count", 0,
			"total", total,
			"has_more", false,
		)
		return ListArchiveCardsResult{
			Items:   []ArchiveCardItem{},
			Total:   total,
			Offset:  offset,
			Limit:   limit,
			HasMore: false,
		}, nil
	}

	items := make([]ArchiveCardItem, 0, len(units))
	missingParentIDs := make([]uint64, 0, len(units))
	missingDurationCandidates := make([]archiveMediaDurationWarmupCandidate, 0, len(units))
	for _, unit := range units {
		coverNodeID := resolveArchiveCoverNodeIDFromMeta(unit.ViewMeta, builtInType)
		if coverNodeID == 0 && unit.CoverNodeID > 0 {
			coverNodeID = unit.CoverNodeID
		}
		if coverNodeID == 0 && builtInType != "AUDIO" {
			missingParentIDs = append(missingParentIDs, unit.ID)
		}
		durationSeconds := resolveNodeMediaDurationFromMeta(unit.MediaViewMeta)
		if builtInType == "VIDEO" && unit.MediaNodeID > 0 && durationSeconds <= 0 {
			missingDurationCandidates = append(missingDurationCandidates, archiveMediaDurationWarmupCandidate{
				MediaNodeID: unit.MediaNodeID,
				ViewMeta:    unit.MediaViewMeta,
			})
		}
		items = append(items, ArchiveCardItem{
			ID:              unit.ID,
			Name:            unit.Name,
			SortOrder:       unit.SortOrder,
			ViewMeta:        strings.TrimSpace(unit.ViewMeta),
			CoverNodeID:     coverNodeID,
			MediaNodeID:     unit.MediaNodeID,
			SubtitleCount:   unit.SubtitleCount,
			DurationSeconds: durationSeconds,
		})
	}

	if builtInType != "AUDIO" && len(missingParentIDs) > 0 {
		coverByParentID, detectErr := u.nodes.DetectFirstImageChildrenByParentIDs(ctx, query.LibraryID, missingParentIDs)
		if detectErr != nil {
			return ListArchiveCardsResult{}, detectErr
		}
		for index := range items {
			if items[index].CoverNodeID > 0 {
				continue
			}
			detectedCover := coverByParentID[items[index].ID]
			if detectedCover == 0 {
				continue
			}
			items[index].CoverNodeID = detectedCover
		}
	}

	// Best-effort warmup to avoid next-round cover scans for comic/asmr archive cards.
	_ = u.warmupArchiveCoverMetaForNodes(ctx, query.LibraryID, builtInType, items)
	if builtInType == "VIDEO" && len(missingDurationCandidates) > 0 {
		u.scheduleArchiveMediaDurationWarmup(ctx, query.LibraryID, missingDurationCandidates)
	}

	hasMore := (offset + len(items)) < total
	slog.DebugContext(ctx, "node.archive.cards.listed",
		"node_id", query.NodeID,
		"library_id", query.LibraryID,
		"built_in_type", builtInType,
		"offset", offset,
		"limit", limit,
		"result_count", len(items),
		"total", total,
		"has_more", hasMore,
	)

	return ListArchiveCardsResult{
		Items:   items,
		Total:   total,
		Offset:  offset,
		Limit:   limit,
		HasMore: hasMore,
	}, nil
}

func (u *NodeUseCase) ListFileStorageKeysByNodeIDs(
	ctx context.Context,
	principal actor.Actor,
	libraryID uint64,
	nodeIDs []uint64,
) (map[uint64]string, error) {
	if libraryID == 0 {
		return map[uint64]string{}, fmt.Errorf("%w: library id is required", ErrInvalidArgument)
	}
	if len(nodeIDs) == 0 {
		return map[uint64]string{}, nil
	}
	if err := u.AuthorizeRead(ctx, principal, libraryID); err != nil {
		return nil, err
	}

	normalized := make([]uint64, 0, len(nodeIDs))
	seen := make(map[uint64]struct{}, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		if nodeID == 0 {
			continue
		}
		if _, exists := seen[nodeID]; exists {
			continue
		}
		seen[nodeID] = struct{}{}
		normalized = append(normalized, nodeID)
	}
	if len(normalized) == 0 {
		return map[uint64]string{}, nil
	}
	return u.nodes.ListStorageKeysByNodeIDs(ctx, libraryID, normalized)
}

// FileStorageInfo 文件节点的存储位置信息。
type FileStorageInfo struct {
	NodeID        uint64
	StorageKey    string
	ProviderAlias string
}

// GetFileStorageProvider 查询单个文件节点的 provider alias。
func (u *NodeUseCase) GetFileStorageProvider(ctx context.Context, nodeID, libraryID uint64) (string, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return "", err
	}
	return u.nodes.GetStorageProviderByNodeID(ctx, nodeID, libraryID)
}

// ListFileStorageInfo 批量查询文件节点的存储位置信息。
func (u *NodeUseCase) ListFileStorageInfo(
	ctx context.Context,
	principal actor.Actor,
	libraryID uint64,
	nodeIDs []uint64,
) ([]FileStorageInfo, error) {
	if libraryID == 0 {
		return nil, fmt.Errorf("%w: library id is required", ErrInvalidArgument)
	}
	if len(nodeIDs) == 0 {
		return []FileStorageInfo{}, nil
	}
	if err := u.AuthorizeRead(ctx, principal, libraryID); err != nil {
		return nil, err
	}

	normalized := normalizePositiveUint64List(nodeIDs)
	if len(normalized) == 0 {
		return []FileStorageInfo{}, nil
	}

	rows, err := u.nodes.ListStorageInfoByNodeIDs(ctx, libraryID, normalized)
	if err != nil {
		return nil, err
	}

	result := make([]FileStorageInfo, 0, len(rows))
	for _, row := range rows {
		result = append(result, FileStorageInfo{
			NodeID:        uint64(row.NodeID),
			StorageKey:    row.StorageKey,
			ProviderAlias: row.ProviderAlias,
		})
	}
	return result, nil
}
