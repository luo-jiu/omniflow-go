package usecase

import (
	"context"
	"sort"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	domain "omniflow-go/internal/domain/resourcemonitor"
)

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
