package cli

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (a *App) runFSMkdir(args []string) error {
	fs := a.newFlagSet("fs mkdir")

	var (
		baseURL   string
		libraryID uint64
		parentID  uint64
		name      string
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Uint64Var(&parentID, "parent-id", 0, "parent node id (optional, default root)")
	fs.StringVar(&name, "name", "", "directory name (required)")
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

	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("`--name` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	created, err := client.CreateNode(context.Background(), CreateNodeRequest{
		Name:      name,
		Type:      0,
		ParentID:  parentID,
		LibraryID: libraryID,
	})
	if err != nil {
		return err
	}

	if jsonOut {
		return a.printJSON(created)
	}
	a.printf("created directory: id=%d name=%s library=%d parent=%d\n", created.ID, created.Name, created.LibraryID, created.ParentID)
	return nil
}

func (a *App) runFSRename(args []string) error {
	fs := a.newFlagSet("fs rename")

	var (
		baseURL string
		nodeID  uint64
		name    string
		jsonOut bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&nodeID, "node-id", 0, "target node id (required)")
	fs.StringVar(&name, "name", "", "new node name (required)")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if nodeID == 0 {
		return errors.New("`--node-id` is required")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("`--name` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	if err := client.RenameNode(context.Background(), nodeID, RenameNodeRequest{Name: name}); err != nil {
		return err
	}

	result := map[string]any{
		"ok":     true,
		"nodeId": nodeID,
		"name":   name,
	}
	if jsonOut {
		return a.printJSON(result)
	}
	a.printf("renamed node: id=%d name=%s\n", nodeID, name)
	return nil
}

func (a *App) runFSMove(args []string) error {
	fs := a.newFlagSet("fs mv")

	var (
		baseURL      string
		libraryID    uint64
		nodeID       uint64
		newParentID  uint64
		beforeNodeID uint64
		name         string
		jsonOut      bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Uint64Var(&nodeID, "node-id", 0, "target node id (required)")
	fs.Uint64Var(&newParentID, "new-parent-id", 0, "target parent node id (required)")
	fs.Uint64Var(&beforeNodeID, "before-node-id", 0, "optional sibling node id to place before")
	fs.StringVar(&name, "name", "", "optional rename while moving")
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
	if nodeID == 0 {
		return errors.New("`--node-id` is required")
	}
	if newParentID == 0 {
		return errors.New("`--new-parent-id` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	if err := client.MoveNode(context.Background(), nodeID, MoveNodeRequest{
		Name:         strings.TrimSpace(name),
		NodeID:       nodeID,
		NewParentID:  newParentID,
		BeforeNodeID: beforeNodeID,
		LibraryID:    libraryID,
	}); err != nil {
		return err
	}

	result := map[string]any{
		"ok":           true,
		"nodeId":       nodeID,
		"libraryId":    libraryID,
		"newParentId":  newParentID,
		"beforeNodeId": beforeNodeID,
	}
	if strings.TrimSpace(name) != "" {
		result["name"] = strings.TrimSpace(name)
	}
	if jsonOut {
		return a.printJSON(result)
	}
	a.printf("moved node: id=%d library=%d new_parent=%d before=%d\n", nodeID, libraryID, newParentID, beforeNodeID)
	return nil
}

func (a *App) runFSRemove(args []string) error {
	fs := a.newFlagSet("fs rm")

	var (
		baseURL   string
		libraryID uint64
		nodeID    uint64
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Uint64Var(&nodeID, "node-id", 0, "target node id (required)")
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
	if nodeID == 0 {
		return errors.New("`--node-id` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	ok, err := client.DeleteNodeTree(context.Background(), nodeID, libraryID)
	if err != nil {
		return err
	}

	result := map[string]any{
		"ok":        ok,
		"nodeId":    nodeID,
		"libraryId": libraryID,
	}
	if jsonOut {
		return a.printJSON(result)
	}
	if ok {
		a.printf("moved node tree to recycle bin: id=%d library=%d\n", nodeID, libraryID)
		return nil
	}
	a.printf("node tree was not moved to recycle bin: id=%d library=%d\n", nodeID, libraryID)
	return nil
}

func (a *App) runFSRecycleList(args []string) error {
	fs := a.newFlagSet("fs recycle ls")

	var (
		baseURL   string
		libraryID uint64
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
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

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	items, err := client.ListRecycleBin(context.Background(), libraryID)
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
		deletedAt := "-"
		if !item.DeletedAt.IsZero() {
			deletedAt = item.DeletedAt.Format(time.RFC3339)
		}
		a.printf(
			"id=%d type=%s name=%s lib=%d parent=%d deleted_at=%s descendants=%d\n",
			item.ID,
			item.Type,
			item.Name,
			item.LibraryID,
			item.ParentID,
			deletedAt,
			item.DeletedDescendantCount,
		)
	}
	return nil
}

func (a *App) runFSRecycleRestore(args []string) error {
	fs := a.newFlagSet("fs recycle restore")

	var (
		baseURL   string
		libraryID uint64
		nodeID    uint64
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Uint64Var(&nodeID, "node-id", 0, "target node id in recycle bin (required)")
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
	if nodeID == 0 {
		return errors.New("`--node-id` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	ok, err := client.RestoreNodeTree(context.Background(), nodeID, libraryID)
	if err != nil {
		return err
	}

	result := map[string]any{
		"ok":        ok,
		"nodeId":    nodeID,
		"libraryId": libraryID,
	}
	if jsonOut {
		return a.printJSON(result)
	}
	if ok {
		a.printf("restored node tree from recycle bin: id=%d library=%d\n", nodeID, libraryID)
		return nil
	}
	a.printf("node tree was not restored: id=%d library=%d\n", nodeID, libraryID)
	return nil
}

func (a *App) runFSRecycleHardDelete(args []string) error {
	fs := a.newFlagSet("fs recycle hard")

	var (
		baseURL   string
		libraryID uint64
		nodeID    uint64
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Uint64Var(&nodeID, "node-id", 0, "target node id in recycle bin (required)")
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
	if nodeID == 0 {
		return errors.New("`--node-id` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	ok, err := client.HardDeleteNodeTree(context.Background(), nodeID, libraryID)
	if err != nil {
		return err
	}

	result := map[string]any{
		"ok":        ok,
		"nodeId":    nodeID,
		"libraryId": libraryID,
	}
	if jsonOut {
		return a.printJSON(result)
	}
	if ok {
		a.printf("hard deleted node tree from recycle bin: id=%d library=%d\n", nodeID, libraryID)
		return nil
	}
	a.printf("node tree was not hard deleted: id=%d library=%d\n", nodeID, libraryID)
	return nil
}

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
