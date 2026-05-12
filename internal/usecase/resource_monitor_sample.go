package usecase

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	domain "omniflow-go/internal/domain/resourcemonitor"
)

const (
	resourceMonitorSampleScopeGlobal  = "global"
	resourceMonitorSampleScopeLibrary = "library"
)

// CaptureSample 显式采集并持久化一条资源监测历史样本。
func (u *ResourceMonitorUseCase) CaptureSample(
	ctx context.Context,
	principal actor.Actor,
	options ...ResourceMonitorSnapshotOptions,
) (domain.Sample, error) {
	if u.repo == nil {
		return domain.Sample{}, errResourceMonitorRepositoryNotConfigured
	}
	libraryID := uint64(0)
	if len(options) > 0 {
		libraryID = options[0].LibraryID
	}
	userID, err := actorIDToUint64(principal)
	if err != nil {
		return domain.Sample{}, err
	}
	if libraryID > 0 {
		owned, err := u.repo.LibraryBelongsToOwner(ctx, userID, libraryID)
		if err != nil {
			return domain.Sample{}, err
		}
		if !owned {
			return domain.Sample{}, ErrNotFound
		}
	}
	snapshot, err := u.Snapshot(ctx, principal, options...)
	if err != nil {
		return domain.Sample{}, err
	}
	sample, err := buildResourceMonitorSample(principal.ID, libraryID, snapshot)
	if err != nil {
		return domain.Sample{}, err
	}
	if len(options) > 0 && options[0].DryRun {
		sample.DryRun = true
		sample.CreatedAt = time.Now().UTC()
		logResourceMonitorSampleCaptured(ctx, sample, true)
		return sample, nil
	}
	saved, err := u.repo.SaveSample(ctx, sample)
	if err != nil {
		return domain.Sample{}, err
	}
	logResourceMonitorSampleCaptured(ctx, saved, false)
	return saved, nil
}

func buildResourceMonitorSample(
	actorID string,
	libraryID uint64,
	snapshot domain.Snapshot,
) (domain.Sample, error) {
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return domain.Sample{}, err
	}

	scope := resourceMonitorSampleScopeGlobal
	if libraryID > 0 {
		scope = resourceMonitorSampleScopeLibrary
	}
	summary := snapshot.Summary
	probes := snapshot.ProbeSummary
	return domain.Sample{
		ActorID:             strings.TrimSpace(actorID),
		Scope:               scope,
		LibraryID:           int64(libraryID),
		GeneratedAt:         snapshot.GeneratedAt,
		ProviderCount:       summary.ProviderCount,
		BucketCount:         summary.BucketCount,
		ObjectCount:         summary.ObjectCount,
		FileRefCount:        summary.FileRefCount,
		PhysicalBytes:       summary.PhysicalBytes,
		VisibleObjectCount:  summary.VisibleObjectCount,
		VisibleFileRefCount: summary.VisibleFileRefCount,
		VisibleBytes:        summary.VisibleBytes,
		RecycleObjectCount:  summary.RecycleObjectCount,
		RecycleFileRefCount: summary.RecycleFileRefCount,
		RecycleBytes:        summary.RecycleBytes,
		OrphanObjectCount:   summary.OrphanObjectCount,
		OrphanBytes:         summary.OrphanBytes,
		UnmatchedCount:      summary.UnmatchedCount,
		LegacyProviderCount: summary.LegacyProviderCount,
		ProbeTotal:          probes.Total,
		ProbeOK:             probes.OK,
		ProbeError:          probes.Error,
		ProbeUnknown:        probes.Unknown,
		DistributionError:   snapshot.DistributionError,
		SnapshotJSON:        string(snapshotJSON),
	}, nil
}

func logResourceMonitorSampleCaptured(ctx context.Context, sample domain.Sample, dryRun bool) {
	slog.InfoContext(ctx, "resource_monitor.sample.captured",
		"sample_id", sample.ID,
		"actor_id", sample.ActorID,
		"scope", sample.Scope,
		"library_id", sample.LibraryID,
		"mode", resolveMutationMode(dryRun),
		"dry_run", dryRun,
		"physical_bytes", sample.PhysicalBytes,
		"object_count", sample.ObjectCount,
		"probe_total", sample.ProbeTotal,
		"probe_ok", sample.ProbeOK,
		"probe_error", sample.ProbeError,
	)
}
