package usecase

import (
	"context"
	"sort"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	domain "omniflow-go/internal/domain/resourcemonitor"
)

// Dashboard 返回当前用户可见资料库范围内的 V2 统计仪表盘快照。
func (u *ResourceMonitorUseCase) Dashboard(
	ctx context.Context,
	principal actor.Actor,
	options ...ResourceMonitorSnapshotOptions,
) (domain.Dashboard, error) {
	if u.repo == nil {
		return domain.Dashboard{}, errResourceMonitorRepositoryNotConfigured
	}
	userID, err := actorIDToUint64(principal)
	if err != nil {
		return domain.Dashboard{}, err
	}

	dashboard := domain.Dashboard{
		GeneratedAt: time.Now().UTC(),
	}
	libraryID := uint64(0)
	if len(options) > 0 {
		libraryID = options[0].LibraryID
	}

	libraryRows, err := u.repo.CountBreakdownLibraries(ctx, userID, libraryID)
	if err != nil {
		if ctx.Err() != nil {
			return domain.Dashboard{}, ctx.Err()
		}
		dashboard.DashboardError = sanitizeProbeError(err)
		dashboard.Statuses = buildBreakdownStatuses(dashboard.Summary)
		return dashboard, nil
	}

	matrixRows, err := u.repo.CountDashboardMatrix(ctx, userID, libraryID)
	if err != nil {
		if ctx.Err() != nil {
			return domain.Dashboard{}, ctx.Err()
		}
		dashboard.DashboardError = sanitizeProbeError(err)
	}

	dashboard.Summary = summarizeBreakdown(libraryRows)
	dashboard.Libraries = buildBreakdownLibraries(libraryRows)
	dashboard.Statuses = buildBreakdownStatuses(dashboard.Summary)
	dashboard.Anomalies = buildBreakdownAnomalies(libraryRows)
	dashboard.FileTypes = buildDashboardFileTypes(matrixRows, dashboard.Summary.PhysicalBytes)
	dashboard.Collections = buildDashboardCollections(matrixRows, dashboard.Summary.PhysicalBytes)
	dashboard.CollectionFileTypeMatrix = buildDashboardMatrix(matrixRows, dashboard.Summary.PhysicalBytes)
	return dashboard, nil
}

func buildDashboardCollections(
	rows []domain.DashboardMatrixRow,
	totalBytes int64,
) []domain.DashboardDimensionItem {
	type aggregate struct {
		builtInType     string
		physicalBytes   int64
		referencedBytes int64
		objectCount     int64
		fileRefCount    int64
	}
	aggregates := make(map[string]aggregate, len(rows))
	for _, row := range rows {
		key := normalizeBreakdownCategoryKey(row.CollectionKey)
		item := aggregates[key]
		if item.builtInType == "" {
			item.builtInType = strings.TrimSpace(row.CollectionBuiltInType)
		}
		item.physicalBytes += row.PhysicalBytes
		item.referencedBytes += row.ReferencedBytes
		item.objectCount += row.ObjectCount
		item.fileRefCount += row.FileRefCount
		aggregates[key] = item
	}

	items := make([]domain.DashboardDimensionItem, 0, len(aggregates))
	for key, item := range aggregates {
		items = append(items, domain.DashboardDimensionItem{
			Key:             key,
			Label:           breakdownCategoryLabel(key),
			BuiltInType:     strings.TrimSpace(item.builtInType),
			PhysicalBytes:   item.physicalBytes,
			ReferencedBytes: item.referencedBytes,
			ObjectCount:     item.objectCount,
			FileRefCount:    item.fileRefCount,
			Percent:         percentOf(item.physicalBytes, totalBytes),
		})
	}
	sortDashboardDimensionItems(items)
	return items
}

func buildDashboardFileTypes(
	rows []domain.DashboardMatrixRow,
	totalBytes int64,
) []domain.DashboardDimensionItem {
	type aggregate struct {
		physicalBytes   int64
		referencedBytes int64
		objectCount     int64
		fileRefCount    int64
	}
	aggregates := make(map[string]aggregate, len(rows))
	for _, row := range rows {
		key := normalizeDashboardFileTypeKey(row.FileTypeKey)
		item := aggregates[key]
		item.physicalBytes += row.PhysicalBytes
		item.referencedBytes += row.ReferencedBytes
		item.objectCount += row.ObjectCount
		item.fileRefCount += row.FileRefCount
		aggregates[key] = item
	}

	items := make([]domain.DashboardDimensionItem, 0, len(aggregates))
	for key, item := range aggregates {
		items = append(items, domain.DashboardDimensionItem{
			Key:             key,
			Label:           dashboardFileTypeLabel(key),
			PhysicalBytes:   item.physicalBytes,
			ReferencedBytes: item.referencedBytes,
			ObjectCount:     item.objectCount,
			FileRefCount:    item.fileRefCount,
			Percent:         percentOf(item.physicalBytes, totalBytes),
		})
	}
	sortDashboardDimensionItems(items)
	return items
}

func buildDashboardMatrix(
	rows []domain.DashboardMatrixRow,
	totalBytes int64,
) []domain.DashboardMatrixItem {
	collectionBytes := make(map[string]int64, len(rows))
	for _, row := range rows {
		key := normalizeBreakdownCategoryKey(row.CollectionKey)
		collectionBytes[key] += row.PhysicalBytes
	}

	items := make([]domain.DashboardMatrixItem, 0, len(rows))
	for _, row := range rows {
		collectionKey := normalizeBreakdownCategoryKey(row.CollectionKey)
		fileTypeKey := normalizeDashboardFileTypeKey(row.FileTypeKey)
		items = append(items, domain.DashboardMatrixItem{
			CollectionKey:         collectionKey,
			CollectionLabel:       breakdownCategoryLabel(collectionKey),
			CollectionBuiltInType: strings.TrimSpace(row.CollectionBuiltInType),
			FileTypeKey:           fileTypeKey,
			FileTypeLabel:         dashboardFileTypeLabel(fileTypeKey),
			PhysicalBytes:         row.PhysicalBytes,
			ReferencedBytes:       row.ReferencedBytes,
			ObjectCount:           row.ObjectCount,
			FileRefCount:          row.FileRefCount,
			PercentOfCollection:   percentOf(row.PhysicalBytes, collectionBytes[collectionKey]),
			PercentOfTotal:        percentOf(row.PhysicalBytes, totalBytes),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].PhysicalBytes != items[j].PhysicalBytes {
			return items[i].PhysicalBytes > items[j].PhysicalBytes
		}
		if items[i].CollectionKey != items[j].CollectionKey {
			return items[i].CollectionKey < items[j].CollectionKey
		}
		return items[i].FileTypeKey < items[j].FileTypeKey
	})
	return items
}

func sortDashboardDimensionItems(items []domain.DashboardDimensionItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].PhysicalBytes != items[j].PhysicalBytes {
			return items[i].PhysicalBytes > items[j].PhysicalBytes
		}
		if items[i].ObjectCount != items[j].ObjectCount {
			return items[i].ObjectCount > items[j].ObjectCount
		}
		return items[i].Key < items[j].Key
	})
}

func normalizeDashboardFileTypeKey(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	switch key {
	case "video", "image", "audio", "text", "archive":
		return key
	default:
		return "unknown"
	}
}

func dashboardFileTypeLabel(key string) string {
	switch normalizeDashboardFileTypeKey(key) {
	case "video":
		return "视频"
	case "image":
		return "图片"
	case "audio":
		return "音频"
	case "text":
		return "文本"
	case "archive":
		return "压缩包"
	default:
		return "未知类型"
	}
}
