package usecase

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	domain "omniflow-go/internal/domain/resourcemonitor"
	"omniflow-go/internal/storage"
)

var errResourceMonitorRepositoryNotConfigured = errors.New("resource monitor repository is not configured")

// ResourceMonitorUseCase 编排资源监测控制台的只读快照。
type ResourceMonitorUseCase struct {
	repo      domain.Repository
	redisRepo domain.RedisProbeRepository
	registry  *storage.StorageRegistry
}

// ResourceMonitorSnapshotOptions 表示资源监测快照的可选范围。
type ResourceMonitorSnapshotOptions struct {
	LibraryID uint64
	DryRun    bool
}

// NewResourceMonitorUseCase 创建资源监测用例。
func NewResourceMonitorUseCase(
	repo domain.Repository,
	redisRepo domain.RedisProbeRepository,
	registry *storage.StorageRegistry,
) *ResourceMonitorUseCase {
	return &ResourceMonitorUseCase{
		repo:      repo,
		redisRepo: redisRepo,
		registry:  registry,
	}
}

// Snapshot 返回当前用户可见资料库范围内的资源分布快照。
func (u *ResourceMonitorUseCase) Snapshot(
	ctx context.Context,
	principal actor.Actor,
	options ...ResourceMonitorSnapshotOptions,
) (domain.Snapshot, error) {
	snapshot, err := u.Distribution(ctx, principal, options...)
	if err != nil {
		return domain.Snapshot{}, err
	}
	probeSnapshot, err := u.Probes(ctx, principal, options...)
	if err != nil {
		return domain.Snapshot{}, err
	}
	snapshot.Probes = probeSnapshot.Probes
	snapshot.ProbeSummary = probeSnapshot.ProbeSummary
	return snapshot, nil
}

// Distribution 返回当前用户可见资料库范围内的资源分布快照。
func (u *ResourceMonitorUseCase) Distribution(
	ctx context.Context,
	principal actor.Actor,
	options ...ResourceMonitorSnapshotOptions,
) (domain.Snapshot, error) {
	if u.repo == nil {
		return domain.Snapshot{}, errResourceMonitorRepositoryNotConfigured
	}
	userID, err := actorIDToUint64(principal)
	if err != nil {
		return domain.Snapshot{}, err
	}

	snapshot := domain.Snapshot{
		GeneratedAt: time.Now().UTC(),
	}

	libraryID := uint64(0)
	if len(options) > 0 {
		libraryID = options[0].LibraryID
	}
	rows, err := u.repo.CountStorageDistribution(ctx, userID, libraryID)
	if err != nil {
		if ctx.Err() != nil {
			return domain.Snapshot{}, ctx.Err()
		}
		snapshot.Storage = []domain.StorageDistributionItem{}
		snapshot.DistributionError = sanitizeProbeError(err)
		return snapshot, nil
	}

	defaultProvider := ""
	if u.registry != nil {
		defaultProvider = strings.TrimSpace(u.registry.DefaultAlias())
	}

	itemsByLocation := make(map[string]domain.StorageDistributionItem, len(rows))
	for _, row := range rows {
		provider := strings.TrimSpace(row.Provider)
		bucket := strings.TrimSpace(row.Bucket)

		item := domain.StorageDistributionItem{
			Provider:            provider,
			Bucket:              bucket,
			ObjectCount:         row.ObjectCount,
			FileRefCount:        row.FileRefCount,
			PhysicalBytes:       row.PhysicalBytes,
			VisibleObjectCount:  row.VisibleObjectCount,
			VisibleFileRefCount: row.VisibleFileRefCount,
			VisibleBytes:        row.VisibleBytes,
			RecycleObjectCount:  row.RecycleObjectCount,
			RecycleFileRefCount: row.RecycleFileRefCount,
			RecycleBytes:        row.RecycleBytes,
			OrphanObjectCount:   row.OrphanObjectCount,
			OrphanBytes:         row.OrphanBytes,
		}
		item = u.enrichStorageDistributionItem(item)
		item.IsDefault = defaultProvider != "" && strings.EqualFold(item.Provider, defaultProvider)

		locationKey := item.Provider + "\x00" + item.Bucket
		merged, exists := itemsByLocation[locationKey]
		if !exists {
			itemsByLocation[locationKey] = item
			continue
		}
		merged.ObjectCount += item.ObjectCount
		merged.FileRefCount += item.FileRefCount
		merged.PhysicalBytes += item.PhysicalBytes
		merged.VisibleObjectCount += item.VisibleObjectCount
		merged.VisibleFileRefCount += item.VisibleFileRefCount
		merged.VisibleBytes += item.VisibleBytes
		merged.RecycleObjectCount += item.RecycleObjectCount
		merged.RecycleFileRefCount += item.RecycleFileRefCount
		merged.RecycleBytes += item.RecycleBytes
		merged.OrphanObjectCount += item.OrphanObjectCount
		merged.OrphanBytes += item.OrphanBytes
		merged.MatchedConfig = merged.MatchedConfig || item.MatchedConfig
		merged.IsDefault = merged.IsDefault || item.IsDefault
		merged.IsLegacyProvider = merged.IsLegacyProvider || item.IsLegacyProvider
		if merged.SourceProvider == "" {
			merged.SourceProvider = item.SourceProvider
		}
		if merged.ProviderType == "" {
			merged.ProviderType = item.ProviderType
		}
		if merged.ProviderLabel == "" {
			merged.ProviderLabel = item.ProviderLabel
		}
		if merged.Endpoint == "" {
			merged.Endpoint = item.Endpoint
		}
		itemsByLocation[locationKey] = merged
	}

	snapshot.Storage = make([]domain.StorageDistributionItem, 0, len(itemsByLocation))
	totalBytes := int64(0)
	providerSet := make(map[string]struct{}, len(itemsByLocation))
	bucketSet := make(map[string]struct{}, len(itemsByLocation))
	for _, item := range itemsByLocation {
		totalBytes += item.PhysicalBytes
		if item.Provider != "" {
			providerSet[item.Provider] = struct{}{}
		}
		if item.Bucket != "" {
			bucketSet[item.Provider+"\x00"+item.Bucket] = struct{}{}
		}
		if !item.MatchedConfig {
			snapshot.Summary.UnmatchedCount++
		}
		if item.IsLegacyProvider {
			snapshot.Summary.LegacyProviderCount++
		}
		snapshot.Storage = append(snapshot.Storage, item)
		snapshot.Summary.ObjectCount += item.ObjectCount
		snapshot.Summary.FileRefCount += item.FileRefCount
		snapshot.Summary.PhysicalBytes += item.PhysicalBytes
		snapshot.Summary.VisibleObjectCount += item.VisibleObjectCount
		snapshot.Summary.VisibleFileRefCount += item.VisibleFileRefCount
		snapshot.Summary.VisibleBytes += item.VisibleBytes
		snapshot.Summary.RecycleObjectCount += item.RecycleObjectCount
		snapshot.Summary.RecycleFileRefCount += item.RecycleFileRefCount
		snapshot.Summary.RecycleBytes += item.RecycleBytes
		snapshot.Summary.OrphanObjectCount += item.OrphanObjectCount
		snapshot.Summary.OrphanBytes += item.OrphanBytes
	}

	for i := range snapshot.Storage {
		if totalBytes <= 0 {
			continue
		}
		percent := float64(snapshot.Storage[i].PhysicalBytes) / float64(totalBytes) * 100
		snapshot.Storage[i].Percent = math.Round(percent*10) / 10
	}

	sort.SliceStable(snapshot.Storage, func(i, j int) bool {
		if snapshot.Storage[i].PhysicalBytes != snapshot.Storage[j].PhysicalBytes {
			return snapshot.Storage[i].PhysicalBytes > snapshot.Storage[j].PhysicalBytes
		}
		if snapshot.Storage[i].ObjectCount != snapshot.Storage[j].ObjectCount {
			return snapshot.Storage[i].ObjectCount > snapshot.Storage[j].ObjectCount
		}
		left := snapshot.Storage[i].Provider + "\x00" + snapshot.Storage[i].Bucket
		right := snapshot.Storage[j].Provider + "\x00" + snapshot.Storage[j].Bucket
		return left < right
	})

	snapshot.Summary.ProviderCount = len(providerSet)
	snapshot.Summary.BucketCount = len(bucketSet)
	return snapshot, nil
}

// Probes 返回当前资源监测探针结果。
func (u *ResourceMonitorUseCase) Probes(
	ctx context.Context,
	principal actor.Actor,
	options ...ResourceMonitorSnapshotOptions,
) (domain.Snapshot, error) {
	if u.repo == nil {
		return domain.Snapshot{}, errResourceMonitorRepositoryNotConfigured
	}
	if _, err := actorIDToUint64(principal); err != nil {
		return domain.Snapshot{}, err
	}

	defaultProvider := ""
	if u.registry != nil {
		defaultProvider = strings.TrimSpace(u.registry.DefaultAlias())
	}
	snapshot := domain.Snapshot{
		GeneratedAt: time.Now().UTC(),
	}
	snapshot.Probes = u.probeTargets(ctx, snapshot.GeneratedAt, defaultProvider)
	snapshot.ProbeSummary = summarizeProbes(snapshot.Probes)
	return snapshot, nil
}

func percentOf(part int64, total int64) float64 {
	if total <= 0 || part <= 0 {
		return 0
	}
	return math.Round(float64(part)/float64(total)*1000) / 10
}

func (u *ResourceMonitorUseCase) enrichStorageDistributionItem(
	item domain.StorageDistributionItem,
) domain.StorageDistributionItem {
	sourceProvider := strings.TrimSpace(item.Provider)
	if u.registry == nil || sourceProvider == "" {
		return item
	}
	cfg, alias, ok := u.registry.ProviderConfigByAlias(sourceProvider)
	if !ok {
		return item
	}
	if u.isLegacyProviderTypeValue(sourceProvider, alias) {
		item.SourceProvider = sourceProvider
		item.IsLegacyProvider = true
	}
	item.Provider = alias
	item.ProviderType = strings.TrimSpace(cfg.Type)
	item.ProviderLabel = strings.TrimSpace(cfg.Label)
	item.Endpoint = strings.TrimSpace(cfg.Endpoint)
	item.MatchedConfig = true
	if item.Bucket == "" {
		item.Bucket = strings.TrimSpace(cfg.Bucket)
	}
	return item
}

func (u *ResourceMonitorUseCase) isLegacyProviderTypeValue(sourceProvider string, matchedAlias string) bool {
	sourceProvider = strings.TrimSpace(sourceProvider)
	if u.registry == nil || sourceProvider == "" {
		return false
	}
	cfg := u.registry.StorageConfig()
	if cfg == nil {
		return false
	}
	if _, ok := cfg.Providers[sourceProvider]; ok {
		return false
	}
	for alias := range cfg.Providers {
		if strings.EqualFold(alias, sourceProvider) {
			return false
		}
	}
	pcfg, ok := cfg.Providers[matchedAlias]
	if !ok {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(pcfg.Type), sourceProvider)
}
