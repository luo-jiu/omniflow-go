package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	domainbrowserbookmark "omniflow-go/internal/domain/browserbookmark"
	"omniflow-go/internal/repository"
)

var errBrowserBookmarkRepositoryNotConfigured = errors.New("browser bookmark repository is not configured")

type ListBrowserBookmarksQuery struct {
	Actor actor.Actor
}

type MatchBrowserBookmarkQuery struct {
	Actor actor.Actor
	URL   string
}

type CreateBrowserBookmarkCommand struct {
	Actor    actor.Actor
	ParentID *uint64
	Kind     string
	Title    string
	URL      string
	IconURL  string
	DryRun   bool
}

type UpdateBrowserBookmarkCommand struct {
	Actor   actor.Actor
	Title   *string
	URL     *string
	IconURL *string
	DryRun  bool
}

type MoveBrowserBookmarkCommand struct {
	Actor    actor.Actor
	ID       uint64
	ParentID *uint64
	BeforeID *uint64
	AfterID  *uint64
	DryRun   bool
}

type DeleteBrowserBookmarkCommand struct {
	Actor  actor.Actor
	ID     uint64
	DryRun bool
}

type ImportBrowserBookmarkItem struct {
	Kind        string
	Title       string
	URL         string
	URLMatchKey string
	IconURL     string
	Children    []ImportBrowserBookmarkItem
}

type ImportBrowserBookmarksCommand struct {
	Actor  actor.Actor
	Items  []ImportBrowserBookmarkItem
	Source string
	DryRun bool
}

type ImportBrowserBookmarksResult struct {
	ImportedCount int `json:"importedCount"`
}

type browserBookmarkRepository interface {
	ListByOwner(ctx context.Context, ownerUserID uint64) ([]domainbrowserbookmark.BrowserBookmark, error)
	ListSiblings(ctx context.Context, ownerUserID uint64, parentID *uint64, excludeID uint64) ([]domainbrowserbookmark.BrowserBookmark, error)
	FindOwnerByID(ctx context.Context, id, ownerUserID uint64) (domainbrowserbookmark.BrowserBookmark, error)
	FindFirstURLByMatchKey(ctx context.Context, ownerUserID uint64, urlMatchKey string) (domainbrowserbookmark.BrowserBookmark, error)
	NextSortOrder(ctx context.Context, ownerUserID uint64, parentID *uint64) (int, error)
	Create(ctx context.Context, input repository.CreateBrowserBookmarkInput) (domainbrowserbookmark.BrowserBookmark, error)
	UpdateOwnerByID(ctx context.Context, id, ownerUserID uint64, input repository.UpdateBrowserBookmarkInput) (domainbrowserbookmark.BrowserBookmark, error)
	MoveOwnerByID(ctx context.Context, id, ownerUserID uint64, parentID *uint64) error
	UpdateSortOrders(ctx context.Context, ownerUserID uint64, orders []repository.BrowserBookmarkSortOrder) error
	SoftDeleteTreeOwnerByID(ctx context.Context, id, ownerUserID uint64) (bool, error)
}

type BrowserBookmarkUseCase struct {
	bookmarks browserBookmarkRepository
	tx        repository.Transactor
	auditLog  audit.Sink
}

func NewBrowserBookmarkUseCase(
	bookmarks browserBookmarkRepository,
	tx repository.Transactor,
	auditLog audit.Sink,
) *BrowserBookmarkUseCase {
	return &BrowserBookmarkUseCase{
		bookmarks: bookmarks,
		tx:        tx,
		auditLog:  auditLog,
	}
}

func (u *BrowserBookmarkUseCase) ListTree(
	ctx context.Context,
	query ListBrowserBookmarksQuery,
) ([]domainbrowserbookmark.BrowserBookmark, error) {
	if err := u.ensureConfigured(); err != nil {
		return nil, err
	}
	ownerUserID, err := actorIDToUint64(query.Actor)
	if err != nil {
		return nil, err
	}

	rows, err := u.bookmarks.ListByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	tree := buildBrowserBookmarkTree(rows)
	slog.DebugContext(ctx, "browser_bookmark.tree.completed",
		"owner_user_id", ownerUserID,
		"result_count", len(rows),
	)
	return tree, nil
}

func (u *BrowserBookmarkUseCase) Match(
	ctx context.Context,
	query MatchBrowserBookmarkQuery,
) (domainbrowserbookmark.MatchResult, error) {
	if err := u.ensureConfigured(); err != nil {
		return domainbrowserbookmark.MatchResult{}, err
	}
	ownerUserID, err := actorIDToUint64(query.Actor)
	if err != nil {
		return domainbrowserbookmark.MatchResult{}, err
	}
	_, matchKey, err := normalizeBookmarkURL(query.URL)
	if err != nil {
		return domainbrowserbookmark.MatchResult{}, err
	}

	row, err := u.bookmarks.FindFirstURLByMatchKey(ctx, ownerUserID, matchKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domainbrowserbookmark.MatchResult{Matched: false}, nil
		}
		return domainbrowserbookmark.MatchResult{}, err
	}
	return domainbrowserbookmark.MatchResult{
		Matched:  true,
		Bookmark: &row,
	}, nil
}

func (u *BrowserBookmarkUseCase) Create(
	ctx context.Context,
	cmd CreateBrowserBookmarkCommand,
) (domainbrowserbookmark.BrowserBookmark, error) {
	if err := u.ensureConfigured(); err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, err
	}
	ownerUserID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, err
	}
	kind, err := normalizeBookmarkKind(cmd.Kind)
	if err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, err
	}
	title, err := normalizeBookmarkTitle(cmd.Title)
	if err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, err
	}
	iconURL, err := normalizeOptionalBookmarkIconURL(cmd.IconURL)
	if err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, err
	}

	var normalizedURL *string
	var matchKey *string
	if kind == domainbrowserbookmark.KindURL {
		urlValue, keyValue, err := normalizeBookmarkURL(cmd.URL)
		if err != nil {
			return domainbrowserbookmark.BrowserBookmark{}, err
		}
		normalizedURL = &urlValue
		matchKey = &keyValue
	}

	var created domainbrowserbookmark.BrowserBookmark
	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		if err := u.ensureParentFolder(txCtx, ownerUserID, cmd.ParentID); err != nil {
			return err
		}
		sortOrder, err := u.bookmarks.NextSortOrder(txCtx, ownerUserID, cmd.ParentID)
		if err != nil {
			return err
		}
		row, err := u.bookmarks.Create(txCtx, repository.CreateBrowserBookmarkInput{
			OwnerUserID: ownerUserID,
			ParentID:    cmd.ParentID,
			Kind:        kind,
			Title:       title,
			URL:         normalizedURL,
			URLMatchKey: matchKey,
			IconURL:     iconURL,
			SortOrder:   sortOrder,
		})
		if err != nil {
			if errors.Is(err, repository.ErrConflict) {
				return ErrConflict
			}
			return err
		}
		created = row
		return nil
	}); err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "browser_bookmark.create", true, map[string]any{
		"bookmark_id":   created.ID,
		"owner_user_id": ownerUserID,
		"kind":          created.Kind,
		"mode":          resolveMutationMode(cmd.DryRun),
		"dry_run":       cmd.DryRun,
	})
	slog.InfoContext(ctx, "browser_bookmark.created",
		"bookmark_id", created.ID,
		"owner_user_id", ownerUserID,
		"kind", created.Kind,
		"dry_run", cmd.DryRun,
	)
	return created, nil
}

func (u *BrowserBookmarkUseCase) Update(
	ctx context.Context,
	id uint64,
	cmd UpdateBrowserBookmarkCommand,
) (domainbrowserbookmark.BrowserBookmark, error) {
	if err := u.ensureConfigured(); err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, err
	}
	if id == 0 {
		return domainbrowserbookmark.BrowserBookmark{}, fmt.Errorf("%w: bookmarkId is required", ErrInvalidArgument)
	}
	ownerUserID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, err
	}

	input := repository.UpdateBrowserBookmarkInput{}
	if cmd.Title != nil {
		title, err := normalizeBookmarkTitle(*cmd.Title)
		if err != nil {
			return domainbrowserbookmark.BrowserBookmark{}, err
		}
		input.Title = &title
	}
	if cmd.IconURL != nil {
		iconURL, err := normalizeOptionalBookmarkIconURL(*cmd.IconURL)
		if err != nil {
			return domainbrowserbookmark.BrowserBookmark{}, err
		}
		if iconURL == nil {
			empty := ""
			input.IconURL = &empty
		} else {
			input.IconURL = iconURL
		}
	}

	var updated domainbrowserbookmark.BrowserBookmark
	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		current, err := u.bookmarks.FindOwnerByID(txCtx, id, ownerUserID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}
		if cmd.URL != nil {
			if current.Kind != domainbrowserbookmark.KindURL {
				return fmt.Errorf("%w: folder url cannot be updated", ErrInvalidArgument)
			}
			urlValue, keyValue, err := normalizeBookmarkURL(*cmd.URL)
			if err != nil {
				return err
			}
			input.URL = &urlValue
			input.URLMatchKey = &keyValue
		}
		if input.Title == nil && input.URL == nil && input.IconURL == nil {
			return fmt.Errorf("%w: title, url or iconUrl is required", ErrInvalidArgument)
		}
		row, err := u.bookmarks.UpdateOwnerByID(txCtx, id, ownerUserID, input)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			if errors.Is(err, repository.ErrConflict) {
				return ErrConflict
			}
			return err
		}
		updated = row
		return nil
	}); err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "browser_bookmark.update", true, map[string]any{
		"bookmark_id":   updated.ID,
		"owner_user_id": ownerUserID,
		"mode":          resolveMutationMode(cmd.DryRun),
		"dry_run":       cmd.DryRun,
	})
	slog.InfoContext(ctx, "browser_bookmark.updated",
		"bookmark_id", updated.ID,
		"owner_user_id", ownerUserID,
		"dry_run", cmd.DryRun,
	)
	return updated, nil
}

func (u *BrowserBookmarkUseCase) Move(
	ctx context.Context,
	cmd MoveBrowserBookmarkCommand,
) (domainbrowserbookmark.BrowserBookmark, error) {
	if err := u.ensureConfigured(); err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, err
	}
	if cmd.ID == 0 {
		return domainbrowserbookmark.BrowserBookmark{}, fmt.Errorf("%w: bookmarkId is required", ErrInvalidArgument)
	}
	if cmd.BeforeID != nil && *cmd.BeforeID == cmd.ID {
		return domainbrowserbookmark.BrowserBookmark{}, fmt.Errorf("%w: beforeId cannot equal bookmarkId", ErrInvalidArgument)
	}
	if cmd.AfterID != nil && *cmd.AfterID == cmd.ID {
		return domainbrowserbookmark.BrowserBookmark{}, fmt.Errorf("%w: afterId cannot equal bookmarkId", ErrInvalidArgument)
	}
	if cmd.BeforeID != nil && cmd.AfterID != nil {
		return domainbrowserbookmark.BrowserBookmark{}, fmt.Errorf("%w: beforeId and afterId are mutually exclusive", ErrInvalidArgument)
	}

	ownerUserID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, err
	}

	var moved domainbrowserbookmark.BrowserBookmark
	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		current, err := u.bookmarks.FindOwnerByID(txCtx, cmd.ID, ownerUserID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}
		if err := u.ensureMoveTarget(txCtx, ownerUserID, current, cmd.ParentID); err != nil {
			return err
		}

		siblings, err := u.bookmarks.ListSiblings(txCtx, ownerUserID, cmd.ParentID, cmd.ID)
		if err != nil {
			return err
		}
		orderedIDs, err := insertBookmarkIntoSiblingOrder(cmd.ID, siblings, cmd.BeforeID, cmd.AfterID)
		if err != nil {
			return err
		}
		if err := u.bookmarks.MoveOwnerByID(txCtx, cmd.ID, ownerUserID, cmd.ParentID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}
		if err := u.bookmarks.UpdateSortOrders(txCtx, ownerUserID, buildBookmarkSortOrders(orderedIDs)); err != nil {
			return err
		}
		row, err := u.bookmarks.FindOwnerByID(txCtx, cmd.ID, ownerUserID)
		if err != nil {
			return err
		}
		moved = row
		return nil
	}); err != nil {
		return domainbrowserbookmark.BrowserBookmark{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "browser_bookmark.move", true, map[string]any{
		"bookmark_id":   moved.ID,
		"owner_user_id": ownerUserID,
		"parent_id":     moved.ParentID,
		"mode":          resolveMutationMode(cmd.DryRun),
		"dry_run":       cmd.DryRun,
	})
	slog.InfoContext(ctx, "browser_bookmark.moved",
		"bookmark_id", moved.ID,
		"owner_user_id", ownerUserID,
		"dry_run", cmd.DryRun,
	)
	return moved, nil
}

func (u *BrowserBookmarkUseCase) Delete(ctx context.Context, cmd DeleteBrowserBookmarkCommand) error {
	if err := u.ensureConfigured(); err != nil {
		return err
	}
	if cmd.ID == 0 {
		return fmt.Errorf("%w: bookmarkId is required", ErrInvalidArgument)
	}
	ownerUserID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return err
	}

	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		ok, err := u.bookmarks.SoftDeleteTreeOwnerByID(txCtx, cmd.ID, ownerUserID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		return nil
	}); err != nil {
		return err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "browser_bookmark.delete", true, map[string]any{
		"bookmark_id":   cmd.ID,
		"owner_user_id": ownerUserID,
		"mode":          resolveMutationMode(cmd.DryRun),
		"dry_run":       cmd.DryRun,
	})
	slog.InfoContext(ctx, "browser_bookmark.deleted",
		"bookmark_id", cmd.ID,
		"owner_user_id", ownerUserID,
		"dry_run", cmd.DryRun,
	)
	return nil
}

func (u *BrowserBookmarkUseCase) ensureConfigured() error {
	if u == nil || u.bookmarks == nil {
		return errBrowserBookmarkRepositoryNotConfigured
	}
	return nil
}

func (u *BrowserBookmarkUseCase) ensureParentFolder(ctx context.Context, ownerUserID uint64, parentID *uint64) error {
	if parentID == nil || *parentID == 0 {
		return nil
	}
	parent, err := u.bookmarks.FindOwnerByID(ctx, *parentID, ownerUserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if parent.Kind != domainbrowserbookmark.KindFolder {
		return fmt.Errorf("%w: parentId must be a folder", ErrInvalidArgument)
	}
	return nil
}

func (u *BrowserBookmarkUseCase) ensureMoveTarget(
	ctx context.Context,
	ownerUserID uint64,
	current domainbrowserbookmark.BrowserBookmark,
	parentID *uint64,
) error {
	if parentID == nil || *parentID == 0 {
		return nil
	}
	if *parentID == current.ID {
		return fmt.Errorf("%w: parentId cannot equal bookmarkId", ErrInvalidArgument)
	}
	if err := u.ensureParentFolder(ctx, ownerUserID, parentID); err != nil {
		return err
	}
	if current.Kind != domainbrowserbookmark.KindFolder {
		return nil
	}
	rows, err := u.bookmarks.ListByOwner(ctx, ownerUserID)
	if err != nil {
		return err
	}
	parentByID := make(map[uint64]uint64, len(rows))
	for _, row := range rows {
		if row.ParentID != nil {
			parentByID[row.ID] = *row.ParentID
		}
	}
	for cursor := *parentID; cursor != 0; cursor = parentByID[cursor] {
		if cursor == current.ID {
			return fmt.Errorf("%w: cannot move folder into its descendant", ErrInvalidArgument)
		}
		next, ok := parentByID[cursor]
		if !ok || next == cursor {
			break
		}
	}
	return nil
}

func normalizeBookmarkKind(raw string) (string, error) {
	kind := strings.ToLower(strings.TrimSpace(raw))
	if kind == "" {
		kind = domainbrowserbookmark.KindURL
	}
	switch kind {
	case domainbrowserbookmark.KindURL, domainbrowserbookmark.KindFolder:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: kind must be url or folder", ErrInvalidArgument)
	}
}

func normalizeBookmarkTitle(raw string) (string, error) {
	title := strings.TrimSpace(raw)
	if title == "" {
		return "", fmt.Errorf("%w: title is required", ErrInvalidArgument)
	}
	if len([]rune(title)) > 255 {
		return "", fmt.Errorf("%w: title length must be <= 255", ErrInvalidArgument)
	}
	return title, nil
}

func normalizeBookmarkURL(raw string) (string, string, error) {
	bookmarkURL := strings.TrimSpace(raw)
	if bookmarkURL == "" {
		return "", "", fmt.Errorf("%w: url is required", ErrInvalidArgument)
	}
	if len(bookmarkURL) > 2048 {
		return "", "", fmt.Errorf("%w: url length must be <= 2048", ErrInvalidArgument)
	}
	parsed, err := url.Parse(bookmarkURL)
	if err != nil || parsed == nil {
		return "", "", fmt.Errorf("%w: url is invalid", ErrInvalidArgument)
	}
	parsed.Scheme = strings.ToLower(strings.TrimSpace(parsed.Scheme))
	parsed.Host = strings.ToLower(strings.TrimSpace(parsed.Host))
	parsed.RawQuery = ""
	parsed.Fragment = ""
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", fmt.Errorf("%w: url must use http or https", ErrInvalidArgument)
	}
	if parsed.Host == "" {
		return "", "", fmt.Errorf("%w: url host is required", ErrInvalidArgument)
	}
	normalized := parsed.String()
	matchKey := strings.TrimRight(normalized, "/")
	if matchKey == "http:" || matchKey == "https:" {
		matchKey = normalized
	}
	return normalized, matchKey, nil
}

func normalizeOptionalBookmarkIconURL(raw string) (*string, error) {
	iconURL := strings.TrimSpace(raw)
	if iconURL == "" {
		return nil, nil
	}
	if len(iconURL) > 2048 {
		return nil, fmt.Errorf("%w: iconUrl length must be <= 2048", ErrInvalidArgument)
	}
	parsed, err := url.Parse(iconURL)
	if err != nil || parsed == nil {
		return nil, fmt.Errorf("%w: iconUrl is invalid", ErrInvalidArgument)
	}
	parsed.Scheme = strings.ToLower(strings.TrimSpace(parsed.Scheme))
	parsed.Host = strings.ToLower(strings.TrimSpace(parsed.Host))
	parsed.Fragment = ""
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%w: iconUrl must use http or https", ErrInvalidArgument)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("%w: iconUrl host is required", ErrInvalidArgument)
	}
	normalized := parsed.String()
	return &normalized, nil
}

func buildBrowserBookmarkTree(rows []domainbrowserbookmark.BrowserBookmark) []domainbrowserbookmark.BrowserBookmark {
	byParentID := make(map[uint64][]domainbrowserbookmark.BrowserBookmark, len(rows))
	roots := make([]domainbrowserbookmark.BrowserBookmark, 0, len(rows))
	for _, row := range rows {
		row.Children = nil
		if row.ParentID == nil {
			roots = append(roots, row)
			continue
		}
		byParentID[*row.ParentID] = append(byParentID[*row.ParentID], row)
	}
	attachBrowserBookmarkChildren(roots, byParentID)
	return roots
}

func attachBrowserBookmarkChildren(
	items []domainbrowserbookmark.BrowserBookmark,
	byParentID map[uint64][]domainbrowserbookmark.BrowserBookmark,
) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortOrder != items[j].SortOrder {
			return items[i].SortOrder < items[j].SortOrder
		}
		return items[i].ID < items[j].ID
	})
	for i := range items {
		children := byParentID[items[i].ID]
		if len(children) == 0 {
			continue
		}
		items[i].Children = children
		attachBrowserBookmarkChildren(items[i].Children, byParentID)
	}
}

func insertBookmarkIntoSiblingOrder(
	id uint64,
	siblings []domainbrowserbookmark.BrowserBookmark,
	beforeID *uint64,
	afterID *uint64,
) ([]uint64, error) {
	orderedIDs := make([]uint64, 0, len(siblings)+1)
	inserted := false
	if beforeID == nil && afterID == nil {
		for _, item := range siblings {
			orderedIDs = append(orderedIDs, item.ID)
		}
		orderedIDs = append(orderedIDs, id)
		return orderedIDs, nil
	}

	for _, item := range siblings {
		if beforeID != nil && item.ID == *beforeID {
			orderedIDs = append(orderedIDs, id)
			inserted = true
		}
		orderedIDs = append(orderedIDs, item.ID)
		if afterID != nil && item.ID == *afterID {
			orderedIDs = append(orderedIDs, id)
			inserted = true
		}
	}
	if !inserted {
		return nil, fmt.Errorf("%w: beforeId/afterId must be a sibling under parentId", ErrInvalidArgument)
	}
	return orderedIDs, nil
}

func buildBookmarkSortOrders(ids []uint64) []repository.BrowserBookmarkSortOrder {
	orders := make([]repository.BrowserBookmarkSortOrder, 0, len(ids))
	for i, id := range ids {
		orders = append(orders, repository.BrowserBookmarkSortOrder{
			ID:        id,
			SortOrder: (i + 1) * 1000,
		})
	}
	return orders
}

func (u *BrowserBookmarkUseCase) withinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if u.tx == nil {
		return fn(ctx)
	}
	return u.tx.WithinTx(ctx, fn)
}

func (u *BrowserBookmarkUseCase) withinMutationTx(ctx context.Context, dryRun bool, fn func(ctx context.Context) error) error {
	if !dryRun {
		return u.withinTx(ctx, fn)
	}
	if u.tx == nil {
		return fmt.Errorf("%w: dry-run requires transaction manager", ErrInvalidArgument)
	}

	err := u.tx.WithinTx(ctx, func(txCtx context.Context) error {
		if err := fn(txCtx); err != nil {
			return err
		}
		return errUsecaseDryRunRollback
	})
	if err != nil && !errors.Is(err, errUsecaseDryRunRollback) {
		return err
	}
	return nil
}

func (u *BrowserBookmarkUseCase) writeAudit(ctx context.Context, principal actor.Actor, action string, success bool, metadata map[string]any) error {
	if u.auditLog == nil {
		return nil
	}
	return u.auditLog.Write(ctx, audit.Event{
		Actor:      principal,
		Action:     action,
		Resource:   "browser_bookmark",
		Success:    success,
		OccurredAt: time.Now().UTC(),
		Metadata:   metadata,
	})
}
