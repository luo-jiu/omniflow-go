package cli

import (
	"context"
	"errors"
	"strings"
)

func (a *App) runFSList(args []string) error {
	fs := a.newFlagSet("fs ls")

	var (
		baseURL   string
		libraryID uint64
		nodeID    uint64
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Uint64Var(&nodeID, "node-id", 0, "node id (required)")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if libraryID == 0 || nodeID == 0 {
		return errors.New("`--library-id` and `--node-id` are required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	items, err := client.ListChildren(context.Background(), nodeID, libraryID)
	if err != nil {
		return err
	}

	if jsonOut {
		return a.printJSON(items)
	}

	if len(items) == 0 {
		a.println("(empty)")
		return nil
	}
	for _, item := range items {
		a.printf("id=%d type=%s name=%s parent=%d\n", item.ID, item.Type, item.Name, item.ParentID)
	}
	return nil
}

func (a *App) runFSSearch(args []string) error {
	fs := a.newFlagSet("fs search")

	var (
		baseURL      string
		libraryID    uint64
		keyword      string
		limit        int
		tagIDsRaw    string
		tagMatchMode string
		jsonOut      bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.StringVar(&keyword, "keyword", "", "search keyword")
	fs.IntVar(&limit, "limit", 20, "search result limit")
	fs.StringVar(&tagIDsRaw, "tag-ids", "", "comma separated tag ids, e.g. 1,2,3")
	fs.StringVar(&tagMatchMode, "tag-match-mode", "", "tag match mode: ANY or ALL")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if libraryID == 0 {
		return errors.New("`--library-id` is required")
	}

	tagIDs, err := parseUint64CSV(tagIDsRaw)
	if err != nil {
		return err
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	items, err := client.SearchNodes(context.Background(), SearchNodesRequest{
		LibraryID:    libraryID,
		Keyword:      strings.TrimSpace(keyword),
		TagIDs:       tagIDs,
		TagMatchMode: strings.TrimSpace(tagMatchMode),
		Limit:        limit,
	})
	if err != nil {
		return err
	}

	if jsonOut {
		return a.printJSON(items)
	}

	if len(items) == 0 {
		a.println("(empty)")
		return nil
	}
	for _, item := range items {
		a.printf("id=%d type=%s name=%s lib=%d parent=%d\n", item.ID, item.Type, item.Name, item.LibraryID, item.ParentID)
	}
	return nil
}
