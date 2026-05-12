package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/config"
	domain "omniflow-go/internal/domain/resourcemonitor"
	"omniflow-go/internal/storage"
)

var errResourceMonitorRepositoryNotConfigured = errors.New("resource monitor repository is not configured")

const (
	resourceMonitorProbeTimeout       = 2 * time.Second
	resourceMonitorProbeParallelism   = 4
	resourceMonitorProbeErrorMaxSize  = 180
	resourceMonitorSampleScopeGlobal  = "global"
	resourceMonitorSampleScopeLibrary = "library"
)

var (
	probeURIUserInfoPattern = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)([^/\s@]+)@`)
	probeSecretKVPattern    = regexp.MustCompile(
		`(?i)\b(password|passwd|pwd|secret|secret_key|access_key|accesskey|` +
			`access_key_id|accesskeyid|token|credential)(\s*[:=]\s*)([^\s&;,]+)`,
	)
	probeAWSKeyPattern = regexp.MustCompile(`\b(AKIA|ASIA)[A-Z0-9]{12,}\b`)
)

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

// Breakdown 返回当前用户可见资料库范围内的细分仪表盘快照。
func (u *ResourceMonitorUseCase) Breakdown(
	ctx context.Context,
	principal actor.Actor,
	options ...ResourceMonitorSnapshotOptions,
) (domain.Breakdown, error) {
	if u.repo == nil {
		return domain.Breakdown{}, errResourceMonitorRepositoryNotConfigured
	}
	userID, err := actorIDToUint64(principal)
	if err != nil {
		return domain.Breakdown{}, err
	}

	breakdown := domain.Breakdown{
		GeneratedAt: time.Now().UTC(),
	}
	libraryID := uint64(0)
	if len(options) > 0 {
		libraryID = options[0].LibraryID
	}

	libraryRows, err := u.repo.CountBreakdownLibraries(ctx, userID, libraryID)
	if err != nil {
		if ctx.Err() != nil {
			return domain.Breakdown{}, ctx.Err()
		}
		breakdown.BreakdownError = sanitizeProbeError(err)
		breakdown.Statuses = buildBreakdownStatuses(breakdown.Summary)
		return breakdown, nil
	}
	categoryRows, err := u.repo.CountBreakdownCategories(ctx, userID, libraryID)
	if err != nil {
		if ctx.Err() != nil {
			return domain.Breakdown{}, ctx.Err()
		}
		breakdown.BreakdownError = sanitizeProbeError(err)
	}

	breakdown.Libraries = buildBreakdownLibraries(libraryRows)
	breakdown.Categories = buildBreakdownCategories(categoryRows)
	breakdown.Summary = summarizeBreakdown(libraryRows)
	breakdown.Statuses = buildBreakdownStatuses(breakdown.Summary)
	breakdown.Anomalies = buildBreakdownAnomalies(libraryRows)
	return breakdown, nil
}

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

func buildBreakdownLibraries(rows []domain.BreakdownLibraryRow) []domain.BreakdownLibraryItem {
	totalBytes := int64(0)
	for _, row := range rows {
		totalBytes += row.PhysicalBytes
	}

	items := make([]domain.BreakdownLibraryItem, 0, len(rows))
	for _, row := range rows {
		if row.ObjectCount == 0 && row.FileRefCount == 0 && row.ArchiveDirectoryCount == 0 {
			continue
		}
		items = append(items, domain.BreakdownLibraryItem{
			LibraryID:             row.LibraryID,
			LibraryName:           strings.TrimSpace(row.LibraryName),
			PhysicalBytes:         row.PhysicalBytes,
			ReferencedBytes:       row.ReferencedBytes,
			ObjectCount:           row.ObjectCount,
			FileRefCount:          row.FileRefCount,
			ArchiveDirectoryCount: row.ArchiveDirectoryCount,
			VisibleBytes:          row.VisibleBytes,
			RecycleBytes:          row.RecycleBytes,
			OrphanBytes:           row.OrphanBytes,
			TopProvider:           strings.TrimSpace(row.TopProvider),
			TopBucket:             strings.TrimSpace(row.TopBucket),
			Percent:               percentOf(row.PhysicalBytes, totalBytes),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].PhysicalBytes != items[j].PhysicalBytes {
			return items[i].PhysicalBytes > items[j].PhysicalBytes
		}
		if items[i].ObjectCount != items[j].ObjectCount {
			return items[i].ObjectCount > items[j].ObjectCount
		}
		if items[i].LibraryName != items[j].LibraryName {
			return items[i].LibraryName < items[j].LibraryName
		}
		return items[i].LibraryID < items[j].LibraryID
	})
	return items
}

func buildBreakdownCategories(rows []domain.BreakdownCategoryRow) []domain.BreakdownCategoryItem {
	totalBytes := int64(0)
	for _, row := range rows {
		totalBytes += row.PhysicalBytes
	}

	items := make([]domain.BreakdownCategoryItem, 0, len(rows))
	for _, row := range rows {
		key := normalizeBreakdownCategoryKey(row.Key)
		items = append(items, domain.BreakdownCategoryItem{
			Key:                   key,
			Label:                 breakdownCategoryLabel(key),
			BuiltInType:           strings.TrimSpace(row.BuiltInType),
			PhysicalBytes:         row.PhysicalBytes,
			ReferencedBytes:       row.ReferencedBytes,
			ObjectCount:           row.ObjectCount,
			FileRefCount:          row.FileRefCount,
			ArchiveDirectoryCount: row.ArchiveDirectoryCount,
			VisibleBytes:          row.VisibleBytes,
			RecycleBytes:          row.RecycleBytes,
			OrphanBytes:           row.OrphanBytes,
			Percent:               percentOf(row.PhysicalBytes, totalBytes),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].PhysicalBytes != items[j].PhysicalBytes {
			return items[i].PhysicalBytes > items[j].PhysicalBytes
		}
		if items[i].ObjectCount != items[j].ObjectCount {
			return items[i].ObjectCount > items[j].ObjectCount
		}
		return items[i].Key < items[j].Key
	})
	return items
}

func summarizeBreakdown(rows []domain.BreakdownLibraryRow) domain.BreakdownSummary {
	var summary domain.BreakdownSummary
	for _, row := range rows {
		summary.PhysicalBytes += row.PhysicalBytes
		summary.ReferencedBytes += row.ReferencedBytes
		summary.ObjectCount += row.ObjectCount
		summary.FileRefCount += row.FileRefCount
		summary.VisibleObjectCount += row.VisibleObjectCount
		summary.VisibleFileRefCount += row.VisibleFileRefCount
		summary.VisibleBytes += row.VisibleBytes
		summary.RecycleObjectCount += row.RecycleObjectCount
		summary.RecycleFileRefCount += row.RecycleFileRefCount
		summary.RecycleBytes += row.RecycleBytes
		summary.OrphanObjectCount += row.OrphanObjectCount
		summary.OrphanBytes += row.OrphanBytes
		summary.ArchiveDirectoryCount += row.ArchiveDirectoryCount
		summary.MultiRefObjectCount += row.MultiRefObjectCount
		summary.MultiRefPhysicalBytes += row.MultiRefPhysicalBytes
		if row.ObjectCount > 0 || row.FileRefCount > 0 || row.ArchiveDirectoryCount > 0 {
			summary.LibraryCount++
		}
	}
	return summary
}

func buildBreakdownStatuses(summary domain.BreakdownSummary) []domain.BreakdownStatusItem {
	return []domain.BreakdownStatusItem{
		{
			Key:           "visible",
			Label:         "可见资源",
			PhysicalBytes: summary.VisibleBytes,
			ObjectCount:   summary.VisibleObjectCount,
			FileRefCount:  summary.VisibleFileRefCount,
			Percent:       percentOf(summary.VisibleBytes, summary.PhysicalBytes),
		},
		{
			Key:           "recycle",
			Label:         "回收站",
			PhysicalBytes: summary.RecycleBytes,
			ObjectCount:   summary.RecycleObjectCount,
			FileRefCount:  summary.RecycleFileRefCount,
			Percent:       percentOf(summary.RecycleBytes, summary.PhysicalBytes),
		},
		{
			Key:           "orphan",
			Label:         "孤儿对象",
			PhysicalBytes: summary.OrphanBytes,
			ObjectCount:   summary.OrphanObjectCount,
			FileRefCount:  0,
			Percent:       percentOf(summary.OrphanBytes, summary.PhysicalBytes),
		},
	}
}

func buildBreakdownAnomalies(rows []domain.BreakdownLibraryRow) []domain.BreakdownAnomalyItem {
	anomalies := make([]domain.BreakdownAnomalyItem, 0, 3)
	recycle := topLibraryBy(rows, func(row domain.BreakdownLibraryRow) int64 {
		return row.RecycleBytes
	})
	if recycle.RecycleBytes > 0 {
		anomalies = append(anomalies, domain.BreakdownAnomalyItem{
			Key:           "top-recycle-library",
			Severity:      "warning",
			Title:         "回收站占用集中",
			Message:       strings.TrimSpace(recycle.LibraryName),
			LibraryID:     recycle.LibraryID,
			PhysicalBytes: recycle.RecycleBytes,
			ObjectCount:   recycle.RecycleObjectCount,
		})
	}
	orphan := topLibraryBy(rows, func(row domain.BreakdownLibraryRow) int64 {
		return row.OrphanBytes
	})
	if orphan.OrphanBytes > 0 {
		anomalies = append(anomalies, domain.BreakdownAnomalyItem{
			Key:           "top-orphan-library",
			Severity:      "danger",
			Title:         "孤儿对象占用",
			Message:       strings.TrimSpace(orphan.LibraryName),
			LibraryID:     orphan.LibraryID,
			PhysicalBytes: orphan.OrphanBytes,
			ObjectCount:   orphan.OrphanObjectCount,
		})
	}
	multiRef := topLibraryBy(rows, func(row domain.BreakdownLibraryRow) int64 {
		return row.MultiRefPhysicalBytes
	})
	if multiRef.MultiRefObjectCount > 0 {
		anomalies = append(anomalies, domain.BreakdownAnomalyItem{
			Key:           "top-multi-ref-library",
			Severity:      "info",
			Title:         "多引用对象",
			Message:       strings.TrimSpace(multiRef.LibraryName),
			LibraryID:     multiRef.LibraryID,
			PhysicalBytes: multiRef.MultiRefPhysicalBytes,
			ObjectCount:   multiRef.MultiRefObjectCount,
		})
	}
	return anomalies
}

func topLibraryBy(
	rows []domain.BreakdownLibraryRow,
	value func(domain.BreakdownLibraryRow) int64,
) domain.BreakdownLibraryRow {
	var top domain.BreakdownLibraryRow
	for _, row := range rows {
		if value(row) > value(top) {
			top = row
		}
	}
	return top
}

func normalizeBreakdownCategoryKey(value string) string {
	key := strings.ToUpper(strings.TrimSpace(value))
	switch key {
	case "DEF", "COMIC", "ASMR", "VIDEO", "AUDIO", "UNKNOWN", "UNCLASSIFIED":
		return key
	case "":
		return "DEF"
	default:
		return "UNKNOWN"
	}
}

func breakdownCategoryLabel(key string) string {
	switch normalizeBreakdownCategoryKey(key) {
	case "DEF":
		return "普通资源"
	case "COMIC":
		return "漫画"
	case "ASMR":
		return "ASMR"
	case "VIDEO":
		return "视频"
	case "AUDIO":
		return "音频"
	case "UNCLASSIFIED":
		return "未归类对象"
	default:
		return "未知类型"
	}
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

func (u *ResourceMonitorUseCase) probeTargets(
	ctx context.Context,
	checkedAt time.Time,
	defaultProvider string,
) []domain.ProbeTarget {
	targets := make([]domain.ProbeTarget, 0, 4)
	targets = append(targets, u.objectStorageProbeTargets(ctx, checkedAt, defaultProvider)...)
	targets = append(targets, u.runProbe(ctx, domain.ProbeTarget{
		Key:       "postgres:primary",
		Kind:      "postgres",
		Label:     "PostgreSQL",
		Status:    domain.ProbeStatusUnknown,
		CheckedAt: checkedAt,
	}, func(probeCtx context.Context) error {
		return u.repo.Ping(probeCtx)
	}))
	targets = append(targets, u.runProbe(ctx, domain.ProbeTarget{
		Key:       "redis:primary",
		Kind:      "redis",
		Label:     "Redis",
		Status:    domain.ProbeStatusUnknown,
		CheckedAt: checkedAt,
	}, func(probeCtx context.Context) error {
		if u.redisRepo == nil {
			return errors.New("redis probe repository is not configured")
		}
		return u.redisRepo.Ping(probeCtx)
	}))
	return targets
}

func (u *ResourceMonitorUseCase) objectStorageProbeTargets(
	ctx context.Context,
	checkedAt time.Time,
	defaultProvider string,
) []domain.ProbeTarget {
	if u.registry == nil {
		return nil
	}
	cfg := u.registry.StorageConfig()
	if cfg == nil {
		return nil
	}
	aliases := make([]string, 0, len(cfg.Providers))
	for alias := range cfg.Providers {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	targets := make([]domain.ProbeTarget, 0, len(aliases))
	if len(aliases) == 0 {
		return targets
	}

	results := make([]domain.ProbeTarget, len(aliases))
	sem := make(chan struct{}, resourceMonitorProbeParallelism)
	var wg sync.WaitGroup
	for index, alias := range aliases {
		index, alias := index, alias
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[index] = u.objectStorageProbeTarget(
				ctx,
				checkedAt,
				defaultProvider,
				alias,
				cfg.Providers[alias],
			)
		}()
	}
	wg.Wait()
	return results
}

func (u *ResourceMonitorUseCase) objectStorageProbeTarget(
	ctx context.Context,
	checkedAt time.Time,
	defaultProvider string,
	alias string,
	pcfg config.ProviderConfig,
) domain.ProbeTarget {
	target := domain.ProbeTarget{
		Key:          "object-storage:" + alias,
		Kind:         "object_storage",
		Label:        probeLabel(alias, pcfg.Label),
		Provider:     alias,
		ProviderType: strings.TrimSpace(pcfg.Type),
		Endpoint:     strings.TrimSpace(pcfg.Endpoint),
		Bucket:       strings.TrimSpace(pcfg.Bucket),
		IsDefault:    defaultProvider != "" && strings.EqualFold(alias, defaultProvider),
		Status:       domain.ProbeStatusUnknown,
		CheckedAt:    checkedAt,
	}
	store, err := u.registry.Get(alias)
	if err != nil || store == nil {
		target.Status = domain.ProbeStatusError
		if err != nil {
			target.Error = sanitizeProbeError(err)
		} else {
			target.Error = "storage provider is not available"
		}
		return target
	}
	return u.runProbe(ctx, target, store.Probe)
}

func (u *ResourceMonitorUseCase) runProbe(
	ctx context.Context,
	target domain.ProbeTarget,
	fn func(context.Context) error,
) domain.ProbeTarget {
	start := time.Now()
	probeCtx, cancel := context.WithTimeout(ctx, resourceMonitorProbeTimeout)
	defer cancel()

	if err := fn(probeCtx); err != nil {
		target.Status = domain.ProbeStatusError
		target.Error = sanitizeProbeError(err)
	} else {
		target.Status = domain.ProbeStatusOK
	}
	target.LatencyMs = time.Since(start).Milliseconds()
	return target
}

func probeLabel(alias string, label string) string {
	label = strings.TrimSpace(label)
	if label != "" {
		return label
	}
	alias = strings.TrimSpace(alias)
	if alias != "" {
		return alias
	}
	return "未命名存储"
}

func sanitizeProbeError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.Join(strings.Fields(err.Error()), " ")
	message = probeURIUserInfoPattern.ReplaceAllString(message, `${1}***@`)
	message = probeSecretKVPattern.ReplaceAllString(message, `${1}${2}***`)
	message = probeAWSKeyPattern.ReplaceAllString(message, `***`)
	if len(message) > resourceMonitorProbeErrorMaxSize {
		return message[:resourceMonitorProbeErrorMaxSize] + "..."
	}
	return message
}

func summarizeProbes(targets []domain.ProbeTarget) domain.ProbeSummary {
	summary := domain.ProbeSummary{Total: len(targets)}
	for _, target := range targets {
		switch target.Status {
		case domain.ProbeStatusOK:
			summary.OK++
		case domain.ProbeStatusError:
			summary.Error++
		default:
			summary.Unknown++
		}
	}
	return summary
}
