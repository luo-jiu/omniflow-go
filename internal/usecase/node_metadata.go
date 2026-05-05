package usecase

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

const (
	viewMetaViewerStateKey       = "__omniflowViewerStateV1"
	viewMetaViewerStateLegacyKey = "__omniflow_viewer_state_v1"
	viewMetaComicArchiveCardKey  = "comicArchiveCard"
	viewMetaComicArchiveCardOld  = "comic_archive_card"
	viewMetaCoverNodeIDKey       = "coverNodeId"
	viewMetaCoverNodeIDLegacyKey = "cover_node_id"
	viewMetaUpdatedAtKey         = "updatedAt"
	nodeMetadataKey              = "__omniflowNodeMetadataV1"
	nodeMetadataMediaKey         = "media"
	nodeMetadataDurationKey      = "durationSeconds"
	nodeMetadataSourceKey        = "source"
	nodeMetadataProbedAtKey      = "probedAt"
)

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

func parsePositiveFloat64(value any) float64 {
	switch typed := value.(type) {
	case float64:
		if typed <= 0 {
			return 0
		}
		return typed
	case float32:
		if typed <= 0 {
			return 0
		}
		return float64(typed)
	case int:
		if typed <= 0 {
			return 0
		}
		return float64(typed)
	case int64:
		if typed <= 0 {
			return 0
		}
		return float64(typed)
	case uint64:
		if typed == 0 {
			return 0
		}
		return float64(typed)
	case uint:
		if typed == 0 {
			return 0
		}
		return float64(typed)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil || parsed <= 0 {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func resolveNodeMediaDurationFromMeta(viewMetaRaw string) float64 {
	meta := parseJSONMap(viewMetaRaw)
	nodeMetaCandidate, ok := meta[nodeMetadataKey]
	if !ok {
		return 0
	}
	nodeMeta, ok := nodeMetaCandidate.(map[string]any)
	if !ok {
		return 0
	}
	mediaCandidate, ok := nodeMeta[nodeMetadataMediaKey]
	if !ok {
		return 0
	}
	media, ok := mediaCandidate.(map[string]any)
	if !ok {
		return 0
	}
	return parsePositiveFloat64(media[nodeMetadataDurationKey])
}

func applyNodeMediaDurationToMeta(viewMetaRaw string, durationSeconds float64) (string, bool) {
	if durationSeconds <= 0 {
		return strings.TrimSpace(viewMetaRaw), false
	}

	meta := parseJSONMap(viewMetaRaw)
	nodeMetaCandidate := meta[nodeMetadataKey]
	nodeMeta, ok := nodeMetaCandidate.(map[string]any)
	if !ok {
		nodeMeta = map[string]any{}
	}
	mediaCandidate := nodeMeta[nodeMetadataMediaKey]
	media, ok := mediaCandidate.(map[string]any)
	if !ok {
		media = map[string]any{}
	}

	current := parsePositiveFloat64(media[nodeMetadataDurationKey])
	if current > 0 && int64(current*1000) == int64(durationSeconds*1000) {
		return strings.TrimSpace(viewMetaRaw), false
	}

	media[nodeMetadataDurationKey] = durationSeconds
	media[nodeMetadataSourceKey] = "ffprobe"
	media[nodeMetadataProbedAtKey] = time.Now().UTC().Format(time.RFC3339Nano)
	nodeMeta[nodeMetadataMediaKey] = media
	meta[nodeMetadataKey] = nodeMeta

	encoded, err := json.Marshal(meta)
	if err != nil {
		return strings.TrimSpace(viewMetaRaw), false
	}
	return string(encoded), true
}

func resolveArchiveCoverNodeIDFromMeta(viewMetaRaw string, builtInType string) uint64 {
	meta := parseJSONMap(viewMetaRaw)
	if builtInType == "ASMR" || builtInType == "VIDEO" || builtInType == "AUDIO" {
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
	if builtInType == "ASMR" || builtInType == "VIDEO" || builtInType == "AUDIO" {
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
