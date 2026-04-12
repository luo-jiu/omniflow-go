package cli

import (
	"context"
	"errors"
	"strings"
)

func (a *App) runBrowserMapList(args []string) error {
	fs := a.newFlagSet("browser-map ls")

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

	items, err := client.ListBrowserFileMappings(context.Background())
	if err != nil {
		return err
	}

	if jsonOut {
		return a.printJSON(items)
	}
	if len(items) == 0 {
		a.println("no browser file mappings")
		return nil
	}
	for _, item := range items {
		a.printf("id=%d ext=.%s url=%s\n", item.ID, item.FileExt, item.SiteURL)
	}
	return nil
}

func (a *App) runBrowserMapResolve(args []string) error {
	fs := a.newFlagSet("browser-map resolve")

	var (
		baseURL string
		fileExt string
		jsonOut bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.StringVar(&fileExt, "ext", "", "file extension without leading dot (required)")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	fileExt = normalizeBrowserMapExt(fileExt)
	if fileExt == "" {
		return errors.New("`--ext` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	item, err := client.ResolveBrowserFileMapping(context.Background(), fileExt)
	if err != nil {
		return err
	}

	if jsonOut {
		return a.printJSON(item)
	}
	a.printf("resolved browser mapping: id=%d ext=.%s url=%s\n", item.ID, item.FileExt, item.SiteURL)
	return nil
}

func (a *App) runBrowserMapCreate(args []string) error {
	fs := a.newFlagSet("browser-map create")

	var (
		baseURL string
		fileExt string
		siteURL string
		dryRun  bool
		jsonOut bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.StringVar(&fileExt, "ext", "", "file extension without leading dot (required)")
	fs.StringVar(&siteURL, "url", "", "site url (required)")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	fileExt = normalizeBrowserMapExt(fileExt)
	if fileExt == "" {
		return errors.New("`--ext` is required")
	}
	siteURL = strings.TrimSpace(siteURL)
	if siteURL == "" {
		return errors.New("`--url` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	item, err := client.CreateBrowserFileMapping(context.Background(), BrowserFileMappingUpsertRequest{
		FileExt: fileExt,
		SiteURL: siteURL,
	}, dryRun)
	if err != nil {
		return err
	}

	if jsonOut {
		if !dryRun {
			return a.printJSON(item)
		}
		return a.printJSON(map[string]any{
			"dryRun": true,
			"result": item,
		})
	}
	if dryRun {
		a.printf("dry-run: browser mapping create validated: ext=.%s url=%s\n", item.FileExt, item.SiteURL)
		return nil
	}
	a.printf("created browser mapping: id=%d ext=.%s url=%s\n", item.ID, item.FileExt, item.SiteURL)
	return nil
}

func (a *App) runBrowserMapUpdate(args []string) error {
	fs := a.newFlagSet("browser-map update")

	var (
		baseURL   string
		mappingID uint64
		fileExt   string
		siteURL   string
		dryRun    bool
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&mappingID, "id", 0, "mapping id (required)")
	fs.StringVar(&fileExt, "ext", "", "file extension without leading dot (required)")
	fs.StringVar(&siteURL, "url", "", "site url (required)")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	if mappingID == 0 {
		return errors.New("`--id` is required")
	}
	fileExt = normalizeBrowserMapExt(fileExt)
	if fileExt == "" {
		return errors.New("`--ext` is required")
	}
	siteURL = strings.TrimSpace(siteURL)
	if siteURL == "" {
		return errors.New("`--url` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	item, err := client.UpdateBrowserFileMapping(context.Background(), mappingID, BrowserFileMappingUpsertRequest{
		FileExt: fileExt,
		SiteURL: siteURL,
	}, dryRun)
	if err != nil {
		return err
	}

	if jsonOut {
		if !dryRun {
			return a.printJSON(item)
		}
		return a.printJSON(map[string]any{
			"dryRun": true,
			"result": item,
		})
	}
	if dryRun {
		a.printf("dry-run: browser mapping update validated: id=%d ext=.%s url=%s\n", mappingID, item.FileExt, item.SiteURL)
		return nil
	}
	a.printf("updated browser mapping: id=%d ext=.%s url=%s\n", item.ID, item.FileExt, item.SiteURL)
	return nil
}

func (a *App) runBrowserMapDelete(args []string) error {
	fs := a.newFlagSet("browser-map rm")

	var (
		baseURL   string
		mappingID uint64
		dryRun    bool
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&mappingID, "id", 0, "mapping id (required)")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	if mappingID == 0 {
		return errors.New("`--id` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	if err := client.DeleteBrowserFileMapping(context.Background(), mappingID, dryRun); err != nil {
		return err
	}

	result := map[string]any{
		"dryRun": dryRun,
		"id":     mappingID,
		"ok":     true,
	}
	if jsonOut {
		return a.printJSON(result)
	}
	if dryRun {
		a.printf("dry-run: browser mapping delete validated: id=%d\n", mappingID)
		return nil
	}
	a.printf("deleted browser mapping: id=%d\n", mappingID)
	return nil
}

func normalizeBrowserMapExt(raw string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(raw, ".")))
}
