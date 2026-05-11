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
	repo     domain.Repository
	registry *storage.StorageRegistry
}

// NewResourceMonitorUseCase 创建资源监测用例。
func NewResourceMonitorUseCase(
	repo domain.Repository,
	registry *storage.StorageRegistry,
) *ResourceMonitorUseCase {
	return &ResourceMonitorUseCase{
		repo:     repo,
		registry: registry,
	}
}

// Snapshot 返回当前用户可见资料库范围内的资源分布快照。
func (u *ResourceMonitorUseCase) Snapshot(
	ctx context.Context,
	principal actor.Actor,
) (domain.Snapshot, error) {
	if u.repo == nil {
		return domain.Snapshot{}, errResourceMonitorRepositoryNotConfigured
	}
	userID, err := actorIDToUint64(principal)
	if err != nil {
		return domain.Snapshot{}, err
	}

	rows, err := u.repo.CountStorageDistribution(ctx, userID)
	if err != nil {
		return domain.Snapshot{}, err
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
			Provider:      provider,
			Bucket:        bucket,
			ObjectCount:   row.ObjectCount,
			FileRefCount:  row.FileRefCount,
			PhysicalBytes: row.PhysicalBytes,
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
		merged.MatchedConfig = merged.MatchedConfig || item.MatchedConfig
		merged.IsDefault = merged.IsDefault || item.IsDefault
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

	snapshot := domain.Snapshot{
		GeneratedAt: time.Now().UTC(),
		Storage:     make([]domain.StorageDistributionItem, 0, len(itemsByLocation)),
	}
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
		snapshot.Storage = append(snapshot.Storage, item)
		snapshot.Summary.ObjectCount += item.ObjectCount
		snapshot.Summary.FileRefCount += item.FileRefCount
		snapshot.Summary.PhysicalBytes += item.PhysicalBytes
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

func (u *ResourceMonitorUseCase) enrichStorageDistributionItem(
	item domain.StorageDistributionItem,
) domain.StorageDistributionItem {
	if u.registry == nil || strings.TrimSpace(item.Provider) == "" {
		return item
	}
	cfg, alias, ok := u.registry.ProviderConfigByAlias(item.Provider)
	if !ok {
		return item
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
