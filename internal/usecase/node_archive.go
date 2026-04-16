package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	domainnode "omniflow-go/internal/domain/node"
)

const (
	viewMetaViewerStateKey       = "__omniflowViewerStateV1"
	viewMetaViewerStateLegacyKey = "__omniflow_viewer_state_v1"
	viewMetaComicArchiveCardKey  = "comicArchiveCard"
	viewMetaComicArchiveCardOld  = "comic_archive_card"
	viewMetaCoverNodeIDKey       = "coverNodeId"
	viewMetaCoverNodeIDLegacyKey = "cover_node_id"
	viewMetaUpdatedAtKey         = "updatedAt"
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
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	SortOrder   int    `json:"sortOrder"`
	ViewMeta    string `json:"viewMeta,omitempty"`
	CoverNodeID uint64 `json:"coverNodeId,omitempty"`
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
	if normalized == "COMIC" || normalized == "ASMR" || normalized == "VIDEO" {
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

func parseJSONMap(raw string) map[string]any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil || result == nil {
		return map[string]any{}
	}
	return result
}

func parsePositiveUint64(value any) uint64 {
	switch typed := value.(type) {
	case float64:
		if typed <= 0 {
			return 0
		}
		return uint64(typed)
	case float32:
		if typed <= 0 {
			return 0
		}
		return uint64(typed)
	case int:
		if typed <= 0 {
			return 0
		}
		return uint64(typed)
	case int64:
		if typed <= 0 {
			return 0
		}
		return uint64(typed)
	case uint64:
		return typed
	case uint:
		return uint64(typed)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil || parsed == 0 {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func resolveArchiveCoverNodeIDFromMeta(viewMetaRaw string, builtInType string) uint64 {
	meta := parseJSONMap(viewMetaRaw)
	if builtInType == "ASMR" || builtInType == "VIDEO" {
		return parsePositiveUint64(meta[viewMetaCoverNodeIDKey])
	}

	viewerStateCandidate, hasViewerState := meta[viewMetaViewerStateKey]
	if !hasViewerState {
		viewerStateCandidate = meta[viewMetaViewerStateLegacyKey]
	}
	viewerState, ok := viewerStateCandidate.(map[string]any)
	if !ok {
		return 0
	}

	comicCardCandidate, hasComicCard := viewerState[viewMetaComicArchiveCardKey]
	if !hasComicCard {
		comicCardCandidate = viewerState[viewMetaComicArchiveCardOld]
	}
	comicCard, ok := comicCardCandidate.(map[string]any)
	if !ok {
		return 0
	}

	coverNodeID := parsePositiveUint64(comicCard[viewMetaCoverNodeIDKey])
	if coverNodeID > 0 {
		return coverNodeID
	}
	return parsePositiveUint64(comicCard[viewMetaCoverNodeIDLegacyKey])
}

func applyArchiveCoverNodeIDToMeta(viewMetaRaw string, builtInType string, coverNodeID uint64) (string, bool) {
	if coverNodeID == 0 {
		return strings.TrimSpace(viewMetaRaw), false
	}

	meta := parseJSONMap(viewMetaRaw)
	if builtInType == "ASMR" || builtInType == "VIDEO" {
		current := parsePositiveUint64(meta[viewMetaCoverNodeIDKey])
		if current == coverNodeID {
			return strings.TrimSpace(viewMetaRaw), false
		}
		meta[viewMetaCoverNodeIDKey] = coverNodeID
		encoded, err := json.Marshal(meta)
		if err != nil {
			return strings.TrimSpace(viewMetaRaw), false
		}
		return string(encoded), true
	}

	viewerStateCandidate := meta[viewMetaViewerStateKey]
	viewerState, ok := viewerStateCandidate.(map[string]any)
	if !ok {
		viewerState = map[string]any{}
	}
	comicCardCandidate := viewerState[viewMetaComicArchiveCardKey]
	comicCard, ok := comicCardCandidate.(map[string]any)
	if !ok {
		comicCard = map[string]any{}
	}

	current := parsePositiveUint64(comicCard[viewMetaCoverNodeIDKey])
	if current == coverNodeID {
		return strings.TrimSpace(viewMetaRaw), false
	}

	comicCard[viewMetaCoverNodeIDKey] = coverNodeID
	comicCard[viewMetaUpdatedAtKey] = time.Now().UTC().Format(time.RFC3339Nano)
	viewerState[viewMetaComicArchiveCardKey] = comicCard
	delete(viewerState, viewMetaComicArchiveCardOld)
	meta[viewMetaViewerStateKey] = viewerState
	delete(meta, viewMetaViewerStateLegacyKey)

	encoded, err := json.Marshal(meta)
	if err != nil {
		return strings.TrimSpace(viewMetaRaw), false
	}
	return string(encoded), true
}

func (u *NodeUseCase) warmupArchiveCoverMetaForNodes(
	ctx context.Context,
	libraryID uint64,
	builtInType string,
	nodes []ArchiveCardItem,
) error {
	normalizedType := normalizeArchiveCardBuiltInType(builtInType)
	if normalizedType == "" || len(nodes) == 0 {
		return nil
	}
	if normalizedType == "VIDEO" {
		return nil
	}

	targetParentIDs := make([]uint64, 0, len(nodes))
	for _, item := range nodes {
		if item.ID == 0 {
			continue
		}
		if resolveArchiveCoverNodeIDFromMeta(item.ViewMeta, normalizedType) > 0 {
			continue
		}
		targetParentIDs = append(targetParentIDs, item.ID)
	}
	if len(targetParentIDs) == 0 {
		return nil
	}

	coverByParentID, err := u.nodes.DetectFirstImageChildrenByParentIDs(ctx, libraryID, targetParentIDs)
	if err != nil {
		return err
	}
	if len(coverByParentID) == 0 {
		return nil
	}

	for _, item := range nodes {
		coverNodeID := coverByParentID[item.ID]
		if coverNodeID == 0 {
			continue
		}
		nextViewMeta, changed := applyArchiveCoverNodeIDToMeta(item.ViewMeta, normalizedType, coverNodeID)
		if !changed {
			continue
		}
		_, updateErr := u.nodes.UpdateNodeFields(ctx, item.ID, libraryID, map[string]any{
			"view_meta":  nextViewMeta,
			"updated_at": time.Now().UTC(),
		})
		if updateErr != nil {
			return updateErr
		}
	}
	return nil
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
	for _, unit := range units {
		coverNodeID := resolveArchiveCoverNodeIDFromMeta(unit.ViewMeta, builtInType)
		if coverNodeID == 0 && builtInType != "VIDEO" {
			missingParentIDs = append(missingParentIDs, unit.ID)
		}
		items = append(items, ArchiveCardItem{
			ID:          unit.ID,
			Name:        unit.Name,
			SortOrder:   unit.SortOrder,
			ViewMeta:    strings.TrimSpace(unit.ViewMeta),
			CoverNodeID: coverNodeID,
		})
	}

	if builtInType != "VIDEO" && len(missingParentIDs) > 0 {
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
