package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	archiveDurationWarmupLimit   = 6
	archiveDurationProbeTimeout  = 4 * time.Second
	archiveDurationWarmupTimeout = archiveDurationProbeTimeout*time.Duration(archiveDurationWarmupLimit) + 10*time.Second
)

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
	if normalizedType == "AUDIO" {
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

type archiveMediaDurationWarmupCandidate struct {
	MediaNodeID uint64
	ViewMeta    string
}

func resolveFFProbePath() (string, bool) {
	configured := strings.TrimSpace(os.Getenv("OMNIFLOW_FFPROBE_PATH"))
	if configured != "" {
		if strings.Contains(configured, "/") {
			if _, err := os.Stat(configured); err == nil {
				return configured, true
			}
			return "", false
		}
		if path, err := exec.LookPath(configured); err == nil {
			return path, true
		}
		return "", false
	}
	path, err := exec.LookPath("ffprobe")
	if err != nil {
		return "", false
	}
	return path, true
}

func probeMediaDurationSeconds(ctx context.Context, ffprobePath string, mediaURL string) (float64, error) {
	probeCtx, cancel := context.WithTimeout(ctx, archiveDurationProbeTimeout)
	defer cancel()

	output, err := exec.CommandContext(
		probeCtx,
		ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		mediaURL,
	).Output()
	if err != nil {
		return 0, err
	}
	duration := parsePositiveFloat64(strings.TrimSpace(string(output)))
	if duration <= 0 {
		return 0, fmt.Errorf("ffprobe returned empty duration")
	}
	return duration, nil
}

func (u *NodeUseCase) scheduleArchiveMediaDurationWarmup(
	ctx context.Context,
	libraryID uint64,
	candidates []archiveMediaDurationWarmupCandidate,
) {
	if u == nil || len(candidates) == 0 {
		return
	}

	copiedCandidates := append([]archiveMediaDurationWarmupCandidate(nil), candidates...)
	go func() {
		warmupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), archiveDurationWarmupTimeout)
		defer cancel()
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.ErrorContext(warmupCtx, "node.archive.media_duration.warmup_panic",
					"library_id", libraryID,
					"error", recovered,
				)
			}
		}()

		u.warmupArchiveMediaDurationForCards(warmupCtx, libraryID, copiedCandidates)
	}()
}

func (u *NodeUseCase) warmupArchiveMediaDurationForCards(
	ctx context.Context,
	libraryID uint64,
	candidates []archiveMediaDurationWarmupCandidate,
) {
	if u == nil || u.registry == nil || len(candidates) == 0 {
		return
	}
	ffprobePath, ok := resolveFFProbePath()
	if !ok {
		slog.DebugContext(ctx, "node.archive.media_duration.ffprobe_unavailable")
		return
	}

	unique := make([]archiveMediaDurationWarmupCandidate, 0, len(candidates))
	seen := map[uint64]struct{}{}
	for _, candidate := range candidates {
		if candidate.MediaNodeID == 0 {
			continue
		}
		if _, exists := seen[candidate.MediaNodeID]; exists {
			continue
		}
		seen[candidate.MediaNodeID] = struct{}{}
		unique = append(unique, candidate)
		if len(unique) >= archiveDurationWarmupLimit {
			break
		}
	}
	if len(unique) == 0 {
		return
	}

	nodeIDs := make([]uint64, 0, len(unique))
	viewMetaByMediaID := make(map[uint64]string, len(unique))
	for _, candidate := range unique {
		nodeIDs = append(nodeIDs, candidate.MediaNodeID)
		viewMetaByMediaID[candidate.MediaNodeID] = candidate.ViewMeta
	}

	storageInfos, err := u.nodes.ListStorageInfoByNodeIDs(ctx, libraryID, nodeIDs)
	if err != nil {
		slog.WarnContext(ctx, "node.archive.media_duration.storage_info_failed",
			"library_id", libraryID,
			"error", err,
		)
		return
	}
	if len(storageInfos) == 0 {
		return
	}

	for _, info := range storageInfos {
		if info.NodeID <= 0 || strings.TrimSpace(info.StorageKey) == "" {
			continue
		}
		mediaNodeID := uint64(info.NodeID)
		store, storeErr := u.registry.Get(info.ProviderAlias)
		if storeErr != nil {
			slog.WarnContext(ctx, "node.archive.media_duration.provider_unavailable",
				"library_id", libraryID,
				"media_node_id", mediaNodeID,
				"provider", info.ProviderAlias,
				"error", storeErr,
			)
			continue
		}
		url, urlErr := store.GetPresignedURL(ctx, info.StorageKey, 5*time.Minute)
		if urlErr != nil {
			slog.WarnContext(ctx, "node.archive.media_duration.link_failed",
				"library_id", libraryID,
				"media_node_id", mediaNodeID,
				"error", urlErr,
			)
			continue
		}
		duration, probeErr := probeMediaDurationSeconds(ctx, ffprobePath, url)
		if probeErr != nil {
			slog.WarnContext(ctx, "node.archive.media_duration.probe_failed",
				"library_id", libraryID,
				"media_node_id", mediaNodeID,
				"error", probeErr,
			)
			continue
		}

		nextViewMeta, changed := applyNodeMediaDurationToMeta(viewMetaByMediaID[mediaNodeID], duration)
		if changed {
			_, updateErr := u.nodes.UpdateNodeFields(ctx, mediaNodeID, libraryID, map[string]any{
				"view_meta":  nextViewMeta,
				"updated_at": time.Now().UTC(),
			})
			if updateErr != nil {
				slog.WarnContext(ctx, "node.archive.media_duration.persist_failed",
					"library_id", libraryID,
					"media_node_id", mediaNodeID,
					"error", updateErr,
				)
				continue
			}
		}
	}
}
