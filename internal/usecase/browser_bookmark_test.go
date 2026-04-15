package usecase

import (
	"context"
	"errors"
	"testing"

	"omniflow-go/internal/actor"
	domainbrowserbookmark "omniflow-go/internal/domain/browserbookmark"
	"omniflow-go/internal/repository"
)

type fakeBrowserBookmarkRepository struct {
	listByOwnerFunc            func(ctx context.Context, ownerUserID uint64) ([]domainbrowserbookmark.BrowserBookmark, error)
	listSiblingsFunc           func(ctx context.Context, ownerUserID uint64, parentID *uint64, excludeID uint64) ([]domainbrowserbookmark.BrowserBookmark, error)
	findOwnerByIDFunc          func(ctx context.Context, id, ownerUserID uint64) (domainbrowserbookmark.BrowserBookmark, error)
	findFirstURLByMatchKeyFunc func(ctx context.Context, ownerUserID uint64, urlMatchKey string) (domainbrowserbookmark.BrowserBookmark, error)
	nextSortOrderFunc          func(ctx context.Context, ownerUserID uint64, parentID *uint64) (int, error)
	createFunc                 func(ctx context.Context, input repository.CreateBrowserBookmarkInput) (domainbrowserbookmark.BrowserBookmark, error)
	updateOwnerByIDFunc        func(ctx context.Context, id, ownerUserID uint64, input repository.UpdateBrowserBookmarkInput) (domainbrowserbookmark.BrowserBookmark, error)
	moveOwnerByIDFunc          func(ctx context.Context, id, ownerUserID uint64, parentID *uint64) error
	updateSortOrdersFunc       func(ctx context.Context, ownerUserID uint64, orders []repository.BrowserBookmarkSortOrder) error
	softDeleteTreeOwnerByID    func(ctx context.Context, id, ownerUserID uint64) (bool, error)
}

func (f *fakeBrowserBookmarkRepository) ListByOwner(ctx context.Context, ownerUserID uint64) ([]domainbrowserbookmark.BrowserBookmark, error) {
	if f.listByOwnerFunc == nil {
		return nil, nil
	}
	return f.listByOwnerFunc(ctx, ownerUserID)
}

func (f *fakeBrowserBookmarkRepository) ListSiblings(
	ctx context.Context,
	ownerUserID uint64,
	parentID *uint64,
	excludeID uint64,
) ([]domainbrowserbookmark.BrowserBookmark, error) {
	if f.listSiblingsFunc == nil {
		return nil, nil
	}
	return f.listSiblingsFunc(ctx, ownerUserID, parentID, excludeID)
}

func (f *fakeBrowserBookmarkRepository) FindOwnerByID(
	ctx context.Context,
	id, ownerUserID uint64,
) (domainbrowserbookmark.BrowserBookmark, error) {
	if f.findOwnerByIDFunc == nil {
		return domainbrowserbookmark.BrowserBookmark{}, repository.ErrNotFound
	}
	return f.findOwnerByIDFunc(ctx, id, ownerUserID)
}

func (f *fakeBrowserBookmarkRepository) FindFirstURLByMatchKey(
	ctx context.Context,
	ownerUserID uint64,
	urlMatchKey string,
) (domainbrowserbookmark.BrowserBookmark, error) {
	if f.findFirstURLByMatchKeyFunc == nil {
		return domainbrowserbookmark.BrowserBookmark{}, repository.ErrNotFound
	}
	return f.findFirstURLByMatchKeyFunc(ctx, ownerUserID, urlMatchKey)
}

func (f *fakeBrowserBookmarkRepository) NextSortOrder(ctx context.Context, ownerUserID uint64, parentID *uint64) (int, error) {
	if f.nextSortOrderFunc == nil {
		return 1000, nil
	}
	return f.nextSortOrderFunc(ctx, ownerUserID, parentID)
}

func (f *fakeBrowserBookmarkRepository) Create(
	ctx context.Context,
	input repository.CreateBrowserBookmarkInput,
) (domainbrowserbookmark.BrowserBookmark, error) {
	if f.createFunc == nil {
		return domainbrowserbookmark.BrowserBookmark{}, nil
	}
	return f.createFunc(ctx, input)
}

func (f *fakeBrowserBookmarkRepository) UpdateOwnerByID(
	ctx context.Context,
	id, ownerUserID uint64,
	input repository.UpdateBrowserBookmarkInput,
) (domainbrowserbookmark.BrowserBookmark, error) {
	if f.updateOwnerByIDFunc == nil {
		return domainbrowserbookmark.BrowserBookmark{}, nil
	}
	return f.updateOwnerByIDFunc(ctx, id, ownerUserID, input)
}

func (f *fakeBrowserBookmarkRepository) MoveOwnerByID(ctx context.Context, id, ownerUserID uint64, parentID *uint64) error {
	if f.moveOwnerByIDFunc == nil {
		return nil
	}
	return f.moveOwnerByIDFunc(ctx, id, ownerUserID, parentID)
}

func (f *fakeBrowserBookmarkRepository) UpdateSortOrders(
	ctx context.Context,
	ownerUserID uint64,
	orders []repository.BrowserBookmarkSortOrder,
) error {
	if f.updateSortOrdersFunc == nil {
		return nil
	}
	return f.updateSortOrdersFunc(ctx, ownerUserID, orders)
}

func (f *fakeBrowserBookmarkRepository) SoftDeleteTreeOwnerByID(ctx context.Context, id, ownerUserID uint64) (bool, error) {
	if f.softDeleteTreeOwnerByID == nil {
		return false, nil
	}
	return f.softDeleteTreeOwnerByID(ctx, id, ownerUserID)
}

func TestNormalizeBookmarkURLDropsQueryAndFragmentFromMatchKey(t *testing.T) {
	t.Parallel()

	normalized, matchKey, err := normalizeBookmarkURL("https://Example.com/path/?utm_source=test#section")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if normalized != "https://example.com/path/" {
		t.Fatalf("unexpected normalized url: %s", normalized)
	}
	if matchKey != "https://example.com/path" {
		t.Fatalf("unexpected match key: %s", matchKey)
	}
}

func TestBrowserBookmarkUseCaseMatchReturnsUnmatchedOnNotFound(t *testing.T) {
	t.Parallel()

	u := NewBrowserBookmarkUseCase(&fakeBrowserBookmarkRepository{
		findFirstURLByMatchKeyFunc: func(ctx context.Context, ownerUserID uint64, urlMatchKey string) (domainbrowserbookmark.BrowserBookmark, error) {
			return domainbrowserbookmark.BrowserBookmark{}, repository.ErrNotFound
		},
	}, nil, nil)

	result, err := u.Match(context.Background(), MatchBrowserBookmarkQuery{
		Actor: actor.Actor{ID: "7", Kind: actor.KindUser},
		URL:   "https://example.com/demo/?from=search#top",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Matched {
		t.Fatalf("expected unmatched result")
	}
	if result.Bookmark != nil {
		t.Fatalf("expected bookmark to be nil")
	}
}

func TestBrowserBookmarkUseCaseMatchUsesNormalizedMatchKey(t *testing.T) {
	t.Parallel()

	u := NewBrowserBookmarkUseCase(&fakeBrowserBookmarkRepository{
		findFirstURLByMatchKeyFunc: func(ctx context.Context, ownerUserID uint64, urlMatchKey string) (domainbrowserbookmark.BrowserBookmark, error) {
			if ownerUserID != 9 {
				t.Fatalf("expected ownerUserID=9, got %d", ownerUserID)
			}
			if urlMatchKey != "https://example.com/path" {
				t.Fatalf("unexpected match key: %s", urlMatchKey)
			}
			rowURL := "https://example.com/path"
			return domainbrowserbookmark.BrowserBookmark{
				ID:          12,
				Kind:        domainbrowserbookmark.KindURL,
				Title:       "Example",
				URL:         &rowURL,
				URLMatchKey: &urlMatchKey,
			}, nil
		},
	}, nil, nil)

	result, err := u.Match(context.Background(), MatchBrowserBookmarkQuery{
		Actor: actor.Actor{ID: "9", Kind: actor.KindUser},
		URL:   "https://example.com/path/?utm_campaign=summer#hero",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Matched || result.Bookmark == nil || result.Bookmark.ID != 12 {
		t.Fatalf("unexpected match result: %+v", result)
	}
}

func TestBrowserBookmarkUseCaseCreateMapsRepositoryConflict(t *testing.T) {
	t.Parallel()

	u := NewBrowserBookmarkUseCase(&fakeBrowserBookmarkRepository{
		nextSortOrderFunc: func(ctx context.Context, ownerUserID uint64, parentID *uint64) (int, error) {
			return 1000, nil
		},
		createFunc: func(ctx context.Context, input repository.CreateBrowserBookmarkInput) (domainbrowserbookmark.BrowserBookmark, error) {
			return domainbrowserbookmark.BrowserBookmark{}, repository.ErrConflict
		},
	}, nil, nil)

	_, err := u.Create(context.Background(), CreateBrowserBookmarkCommand{
		Actor:  actor.Actor{ID: "3", Kind: actor.KindUser},
		Kind:   domainbrowserbookmark.KindURL,
		Title:  "Example",
		URL:    "https://example.com/path?utm=1",
		DryRun: false,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestBrowserBookmarkUseCaseDeleteMapsMissingBookmark(t *testing.T) {
	t.Parallel()

	u := NewBrowserBookmarkUseCase(&fakeBrowserBookmarkRepository{
		softDeleteTreeOwnerByID: func(ctx context.Context, id, ownerUserID uint64) (bool, error) {
			return false, nil
		},
	}, nil, nil)

	err := u.Delete(context.Background(), DeleteBrowserBookmarkCommand{
		Actor: actor.Actor{ID: "5", Kind: actor.KindUser},
		ID:    99,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestBrowserBookmarkUseCaseImportAppendsHierarchy(t *testing.T) {
	t.Parallel()

	var created []repository.CreateBrowserBookmarkInput
	u := NewBrowserBookmarkUseCase(&fakeBrowserBookmarkRepository{
		nextSortOrderFunc: func(ctx context.Context, ownerUserID uint64, parentID *uint64) (int, error) {
			if parentID == nil {
				return 3000, nil
			}
			return 1000, nil
		},
		createFunc: func(ctx context.Context, input repository.CreateBrowserBookmarkInput) (domainbrowserbookmark.BrowserBookmark, error) {
			created = append(created, input)
			id := uint64(len(created))
			return domainbrowserbookmark.BrowserBookmark{
				ID:          id,
				OwnerUserID: input.OwnerUserID,
				ParentID:    input.ParentID,
				Kind:        input.Kind,
				Title:       input.Title,
				URL:         input.URL,
				URLMatchKey: input.URLMatchKey,
				IconURL:     input.IconURL,
				SortOrder:   input.SortOrder,
			}, nil
		},
	}, nil, nil)

	result, err := u.Import(context.Background(), ImportBrowserBookmarksCommand{
		Actor: actor.Actor{ID: "3", Kind: actor.KindUser},
		Items: []ImportBrowserBookmarkItem{
			{
				Kind:  domainbrowserbookmark.KindFolder,
				Title: "Work",
				Children: []ImportBrowserBookmarkItem{
					{
						Kind:  domainbrowserbookmark.KindURL,
						Title: "Docs",
						URL:   "https://example.com/docs?utm=1",
					},
				},
			},
			{
				Kind:  domainbrowserbookmark.KindURL,
				Title: "Home",
				URL:   "https://example.com/",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ImportedCount != 3 {
		t.Fatalf("expected importedCount=3, got %d", result.ImportedCount)
	}
	if len(created) != 3 {
		t.Fatalf("expected 3 creates, got %d", len(created))
	}
	if created[0].SortOrder != 3000 || created[1].SortOrder != 1000 || created[2].SortOrder != 4000 {
		t.Fatalf("unexpected sort orders: %+v", created)
	}
	if created[1].ParentID == nil || *created[1].ParentID != 1 {
		t.Fatalf("expected child to be created under folder id=1, got %+v", created[1].ParentID)
	}
	if created[1].URLMatchKey == nil || *created[1].URLMatchKey != "https://example.com/docs" {
		t.Fatalf("expected normalized child match key, got %+v", created[1].URLMatchKey)
	}
}

func TestBrowserBookmarkUseCaseImportRejectsFolderURL(t *testing.T) {
	t.Parallel()

	u := NewBrowserBookmarkUseCase(&fakeBrowserBookmarkRepository{}, nil, nil)
	_, err := u.Import(context.Background(), ImportBrowserBookmarksCommand{
		Actor: actor.Actor{ID: "3", Kind: actor.KindUser},
		Items: []ImportBrowserBookmarkItem{
			{
				Kind:  domainbrowserbookmark.KindFolder,
				Title: "Broken",
				URL:   "https://example.com",
			},
		},
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got %v", err)
	}
}
