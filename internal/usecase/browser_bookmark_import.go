package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	domainbrowserbookmark "omniflow-go/internal/domain/browserbookmark"
	"omniflow-go/internal/repository"
)

const browserBookmarkImportSortOrderStep = 1000

func (u *BrowserBookmarkUseCase) Import(
	ctx context.Context,
	cmd ImportBrowserBookmarksCommand,
) (ImportBrowserBookmarksResult, error) {
	if err := u.ensureConfigured(); err != nil {
		return ImportBrowserBookmarksResult{}, err
	}
	ownerUserID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return ImportBrowserBookmarksResult{}, err
	}
	items, err := normalizeImportBrowserBookmarkItems(cmd.Items)
	if err != nil {
		return ImportBrowserBookmarksResult{}, err
	}
	if len(items) == 0 {
		return ImportBrowserBookmarksResult{}, fmt.Errorf("%w: items is required", ErrInvalidArgument)
	}

	result := ImportBrowserBookmarksResult{}
	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		importedCount, err := u.importBrowserBookmarkChildren(txCtx, ownerUserID, nil, items)
		if err != nil {
			return err
		}
		result.ImportedCount = importedCount
		return nil
	}); err != nil {
		return ImportBrowserBookmarksResult{}, err
	}

	metadata := map[string]any{
		"owner_user_id":  ownerUserID,
		"imported_count": result.ImportedCount,
		"mode":           resolveMutationMode(cmd.DryRun),
		"dry_run":        cmd.DryRun,
	}
	if source := strings.TrimSpace(cmd.Source); source != "" {
		metadata["source"] = source
	}
	_ = u.writeAudit(ctx, cmd.Actor, "browser_bookmark.import", true, metadata)
	slog.InfoContext(ctx, "browser_bookmark.imported",
		"owner_user_id", ownerUserID,
		"imported_count", result.ImportedCount,
		"source", strings.TrimSpace(cmd.Source),
		"dry_run", cmd.DryRun,
	)
	return result, nil
}

func (u *BrowserBookmarkUseCase) importBrowserBookmarkChildren(
	ctx context.Context,
	ownerUserID uint64,
	parentID *uint64,
	items []ImportBrowserBookmarkItem,
) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	sortOrder, err := u.bookmarks.NextSortOrder(ctx, ownerUserID, parentID)
	if err != nil {
		return 0, err
	}

	importedCount := 0
	nextSortOrder := sortOrder
	for _, item := range items {
		created, err := u.bookmarks.Create(ctx, repository.CreateBrowserBookmarkInput{
			OwnerUserID: ownerUserID,
			ParentID:    parentID,
			Kind:        item.Kind,
			Title:       item.Title,
			URL:         bookmarkStringPtr(item.URL),
			URLMatchKey: bookmarkStringPtr(item.URLMatchKey),
			IconURL:     bookmarkStringPtr(item.IconURL),
			SortOrder:   nextSortOrder,
		})
		if err != nil {
			if err == repository.ErrConflict {
				return 0, ErrConflict
			}
			return 0, err
		}
		importedCount++
		if len(item.Children) > 0 {
			childCount, err := u.importBrowserBookmarkChildren(ctx, ownerUserID, &created.ID, item.Children)
			if err != nil {
				return 0, err
			}
			importedCount += childCount
		}
		nextSortOrder += browserBookmarkImportSortOrderStep
	}
	return importedCount, nil
}

func normalizeImportBrowserBookmarkItems(items []ImportBrowserBookmarkItem) ([]ImportBrowserBookmarkItem, error) {
	normalized := make([]ImportBrowserBookmarkItem, 0, len(items))
	for _, item := range items {
		normalizedItem, err := normalizeImportBrowserBookmarkItem(item)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, normalizedItem)
	}
	return normalized, nil
}

func normalizeImportBrowserBookmarkItem(item ImportBrowserBookmarkItem) (ImportBrowserBookmarkItem, error) {
	kind, err := normalizeBookmarkKind(item.Kind)
	if err != nil {
		return ImportBrowserBookmarkItem{}, err
	}
	title, err := normalizeBookmarkTitle(item.Title)
	if err != nil {
		return ImportBrowserBookmarkItem{}, err
	}

	normalized := ImportBrowserBookmarkItem{
		Kind:  kind,
		Title: title,
	}
	if kind == domainbrowserbookmark.KindURL {
		urlValue, matchKey, err := normalizeBookmarkURL(item.URL)
		if err != nil {
			return ImportBrowserBookmarkItem{}, err
		}
		normalized.URL = urlValue
		normalized.URLMatchKey = matchKey
	} else if strings.TrimSpace(item.URL) != "" {
		return ImportBrowserBookmarkItem{}, fmt.Errorf("%w: folder item cannot contain url", ErrInvalidArgument)
	}

	iconURL, err := normalizeOptionalBookmarkIconURL(item.IconURL)
	if err != nil {
		return ImportBrowserBookmarkItem{}, err
	}
	if iconURL != nil {
		normalized.IconURL = *iconURL
	}
	if len(item.Children) == 0 {
		return normalized, nil
	}
	if kind != domainbrowserbookmark.KindFolder {
		return ImportBrowserBookmarkItem{}, fmt.Errorf("%w: url item cannot contain children", ErrInvalidArgument)
	}
	children, err := normalizeImportBrowserBookmarkItems(item.Children)
	if err != nil {
		return ImportBrowserBookmarkItem{}, err
	}
	normalized.Children = children
	return normalized, nil
}

func bookmarkStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
