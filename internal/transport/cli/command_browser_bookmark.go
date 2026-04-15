package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
)

func (a *App) runBrowserBookmarkTree(args []string) error {
	fs := a.newFlagSet("browser-bookmark tree")

	var (
		baseURL string
		jsonOut bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	items, err := client.ListBrowserBookmarksTree(context.Background())
	if err != nil {
		return err
	}
	if jsonOut {
		return a.printJSON(items)
	}
	if len(items) == 0 {
		a.println("no browser bookmarks")
		return nil
	}
	printBrowserBookmarkTree(a, items, 0)
	return nil
}

func (a *App) runBrowserBookmarkMatch(args []string) error {
	fs := a.newFlagSet("browser-bookmark match")

	var (
		baseURL string
		rawURL  string
		jsonOut bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.StringVar(&rawURL, "url", "", "browser url to match (required)")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return errors.New("`--url` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	result, err := client.MatchBrowserBookmark(context.Background(), rawURL)
	if err != nil {
		return err
	}
	if jsonOut {
		return a.printJSON(result)
	}
	if !result.Matched || result.Bookmark == nil {
		a.println("no matched browser bookmark")
		return nil
	}
	a.printf("matched browser bookmark: id=%d kind=%s title=%s\n", result.Bookmark.ID, result.Bookmark.Kind, result.Bookmark.Title)
	return nil
}

func (a *App) runBrowserBookmarkCreate(args []string) error {
	fs := a.newFlagSet("browser-bookmark create")

	var (
		baseURL  string
		parentID uint64
		kind     string
		title    string
		rawURL   string
		iconURL  string
		dryRun   bool
		jsonOut  bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&parentID, "parent-id", 0, "parent bookmark folder id")
	fs.StringVar(&kind, "kind", "url", "bookmark kind: url or folder")
	fs.StringVar(&title, "title", "", "bookmark title (required)")
	fs.StringVar(&rawURL, "url", "", "bookmark url for url kind")
	fs.StringVar(&iconURL, "icon-url", "", "bookmark icon url")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("`--title` is required")
	}

	var parentIDPtr *uint64
	if parentID > 0 {
		parentIDPtr = &parentID
	}

	item, err := a.createBrowserBookmark(baseURL, BrowserBookmarkCreateRequest{
		ParentID: parentIDPtr,
		Kind:     strings.TrimSpace(kind),
		Title:    title,
		URL:      strings.TrimSpace(rawURL),
		IconURL:  strings.TrimSpace(iconURL),
	}, dryRun)
	if err != nil {
		return err
	}

	if jsonOut {
		return printDryRunJSON(a, dryRun, item)
	}
	if dryRun {
		a.printf("dry-run: browser bookmark create validated: title=%s kind=%s\n", item.Title, item.Kind)
		return nil
	}
	a.printf("created browser bookmark: id=%d kind=%s title=%s\n", item.ID, item.Kind, item.Title)
	return nil
}

func (a *App) runBrowserBookmarkUpdate(args []string) error {
	fs := a.newFlagSet("browser-bookmark update")

	var (
		baseURL    string
		bookmarkID uint64
		title      string
		rawURL     string
		iconURL    string
		clearIcon  bool
		dryRun     bool
		jsonOut    bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&bookmarkID, "id", 0, "bookmark id (required)")
	fs.StringVar(&title, "title", "", "bookmark title")
	fs.StringVar(&rawURL, "url", "", "bookmark url")
	fs.StringVar(&iconURL, "icon-url", "", "bookmark icon url; empty string clears the icon")
	fs.BoolVar(&clearIcon, "clear-icon", false, "clear bookmark icon")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if bookmarkID == 0 {
		return errors.New("`--id` is required")
	}

	var titlePtr *string
	if fs.Lookup("title").Value.String() != "" {
		trimmed := strings.TrimSpace(title)
		titlePtr = &trimmed
	}
	var urlPtr *string
	if fs.Lookup("url").Value.String() != "" {
		trimmed := strings.TrimSpace(rawURL)
		urlPtr = &trimmed
	}
	var iconURLPtr *string
	if clearIcon {
		empty := ""
		iconURLPtr = &empty
	} else if fs.Lookup("icon-url").Value.String() != "" {
		trimmed := strings.TrimSpace(iconURL)
		iconURLPtr = &trimmed
	}
	if titlePtr == nil && urlPtr == nil && iconURLPtr == nil {
		return errors.New("at least one of `--title`, `--url`, `--icon-url`, or `--clear-icon` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	item, err := client.UpdateBrowserBookmark(context.Background(), bookmarkID, BrowserBookmarkUpdateRequest{
		Title:   titlePtr,
		URL:     urlPtr,
		IconURL: iconURLPtr,
	}, dryRun)
	if err != nil {
		return err
	}
	if jsonOut {
		return printDryRunJSON(a, dryRun, item)
	}
	if dryRun {
		a.printf("dry-run: browser bookmark update validated: id=%d title=%s\n", bookmarkID, item.Title)
		return nil
	}
	a.printf("updated browser bookmark: id=%d title=%s\n", item.ID, item.Title)
	return nil
}

func (a *App) runBrowserBookmarkMove(args []string) error {
	fs := a.newFlagSet("browser-bookmark move")

	var (
		baseURL    string
		bookmarkID uint64
		parentID   uint64
		beforeID   uint64
		afterID    uint64
		dryRun     bool
		jsonOut    bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&bookmarkID, "id", 0, "bookmark id (required)")
	fs.Uint64Var(&parentID, "parent-id", 0, "target parent folder id")
	fs.Uint64Var(&beforeID, "before-id", 0, "insert before sibling id")
	fs.Uint64Var(&afterID, "after-id", 0, "insert after sibling id")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if bookmarkID == 0 {
		return errors.New("`--id` is required")
	}

	var parentIDPtr *uint64
	var beforeIDPtr *uint64
	var afterIDPtr *uint64
	if parentID > 0 {
		parentIDPtr = &parentID
	}
	if beforeID > 0 {
		beforeIDPtr = &beforeID
	}
	if afterID > 0 {
		afterIDPtr = &afterID
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	item, err := client.MoveBrowserBookmark(context.Background(), bookmarkID, BrowserBookmarkMoveRequest{
		ParentID: parentIDPtr,
		BeforeID: beforeIDPtr,
		AfterID:  afterIDPtr,
	}, dryRun)
	if err != nil {
		return err
	}
	if jsonOut {
		return printDryRunJSON(a, dryRun, item)
	}
	if dryRun {
		a.printf("dry-run: browser bookmark move validated: id=%d title=%s\n", bookmarkID, item.Title)
		return nil
	}
	a.printf("moved browser bookmark: id=%d title=%s\n", item.ID, item.Title)
	return nil
}

func (a *App) runBrowserBookmarkDelete(args []string) error {
	fs := a.newFlagSet("browser-bookmark rm")

	var (
		baseURL    string
		bookmarkID uint64
		dryRun     bool
		jsonOut    bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&bookmarkID, "id", 0, "bookmark id (required)")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if bookmarkID == 0 {
		return errors.New("`--id` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}
	if err := client.DeleteBrowserBookmark(context.Background(), bookmarkID, dryRun); err != nil {
		return err
	}

	if jsonOut {
		return printDryRunJSON(a, dryRun, map[string]any{"id": bookmarkID})
	}
	if dryRun {
		a.printf("dry-run: browser bookmark delete validated: id=%d\n", bookmarkID)
		return nil
	}
	a.printf("deleted browser bookmark: id=%d\n", bookmarkID)
	return nil
}

func (a *App) runBrowserBookmarkImport(args []string) error {
	fs := a.newFlagSet("browser-bookmark import")

	var (
		baseURL  string
		filePath string
		source   string
		dryRun   bool
		jsonOut  bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.StringVar(&filePath, "file", "", "JSON file path containing import payload (required)")
	fs.StringVar(&source, "source", "", "import source label")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return errors.New("`--file` is required")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	var req BrowserBookmarkImportRequest
	if err := json.Unmarshal(content, &req); err != nil {
		return err
	}
	if strings.TrimSpace(source) != "" {
		req.Source = strings.TrimSpace(source)
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}
	result, err := client.ImportBrowserBookmarks(context.Background(), req, dryRun)
	if err != nil {
		return err
	}

	if jsonOut {
		return printDryRunJSON(a, dryRun, result)
	}
	if dryRun {
		a.printf("dry-run: browser bookmark import validated: imported=%d\n", result.ImportedCount)
		return nil
	}
	a.printf("imported browser bookmarks: count=%d\n", result.ImportedCount)
	return nil
}

func (a *App) createBrowserBookmark(baseURL string, req BrowserBookmarkCreateRequest, dryRun bool) (BrowserBookmark, error) {
	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return BrowserBookmark{}, err
	}
	return client.CreateBrowserBookmark(context.Background(), req, dryRun)
}

func printBrowserBookmarkTree(a *App, items []BrowserBookmark, depth int) {
	prefix := strings.Repeat("  ", depth)
	for _, item := range items {
		target := ""
		if item.URL != nil && strings.TrimSpace(*item.URL) != "" {
			target = " -> " + strings.TrimSpace(*item.URL)
		}
		a.printf("%s- [%s] %s%s\n", prefix, item.Kind, item.Title, target)
		if len(item.Children) > 0 {
			printBrowserBookmarkTree(a, item.Children, depth+1)
		}
	}
}

func printDryRunJSON(a *App, dryRun bool, result any) error {
	if !dryRun {
		return a.printJSON(result)
	}
	return a.printJSON(map[string]any{
		"dryRun": true,
		"result": result,
	})
}
