package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (a *App) runFSMkdir(args []string) error {
	fs := a.newFlagSet("fs mkdir")

	var (
		baseURL    string
		libraryID  uint64
		parentID   uint64
		parentPath string
		name       string
		dryRun     bool
		jsonOut    bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Uint64Var(&parentID, "parent-id", 0, "parent node id (optional, default root)")
	fs.StringVar(&parentPath, "parent-path", "", "parent path from root, e.g. /docs")
	fs.StringVar(&name, "name", "", "directory name (required)")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
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

	parentPath = strings.TrimSpace(parentPath)
	if parentID > 0 && parentPath != "" {
		return errors.New("`--parent-id` and `--parent-path` cannot be used together")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	if parentPath != "" {
		_, parentNode, err := resolveNodeByPath(context.Background(), client, libraryID, parentPath)
		if err != nil {
			return err
		}
		if parentNode.Type != "dir" {
			return fmt.Errorf("`--parent-path` must resolve to a directory, got type=%s", parentNode.Type)
		}
		parentID = parentNode.ID
	}

	created, err := client.CreateNode(context.Background(), CreateNodeRequest{
		Name:      name,
		Type:      0,
		ParentID:  parentID,
		LibraryID: libraryID,
	}, dryRun)
	if err != nil {
		return err
	}

	if jsonOut {
		if !dryRun {
			// 兼容既有脚本：非 dry-run 维持历史输出（仅 node 对象）。
			return a.printJSON(created)
		}
		return a.printJSON(map[string]any{
			"dryRun": dryRun,
			"node":   created,
		})
	}
	if dryRun {
		a.printf("dry-run: create directory request validated: name=%s library=%d parent=%d\n", created.Name, created.LibraryID, created.ParentID)
		return nil
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
		dryRun  bool
		jsonOut bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&nodeID, "node-id", 0, "target node id (required)")
	fs.StringVar(&name, "name", "", "new node name (required)")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
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

	if err := client.RenameNode(context.Background(), nodeID, RenameNodeRequest{Name: name}, dryRun); err != nil {
		return err
	}

	result := map[string]any{
		"dryRun": dryRun,
		"ok":     true,
		"nodeId": nodeID,
		"name":   name,
	}
	if jsonOut {
		return a.printJSON(result)
	}
	if dryRun {
		a.printf("dry-run: rename request validated: id=%d name=%s\n", nodeID, name)
		return nil
	}
	a.printf("renamed node: id=%d name=%s\n", nodeID, name)
	return nil
}

func (a *App) runFSMove(args []string) error {
	fs := a.newFlagSet("fs mv")

	var (
		baseURL       string
		libraryID     uint64
		nodeID        uint64
		nodePath      string
		newParentID   uint64
		newParentPath string
		beforeNodeID  uint64
		name          string
		dryRun        bool
		jsonOut       bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Uint64Var(&nodeID, "node-id", 0, "target node id (required)")
	fs.StringVar(&nodePath, "node-path", "", "target node path from root, e.g. /docs/a.txt")
	fs.Uint64Var(&newParentID, "new-parent-id", 0, "target parent node id (required)")
	fs.StringVar(&newParentPath, "new-parent-path", "", "target parent path from root, e.g. /docs")
	fs.Uint64Var(&beforeNodeID, "before-node-id", 0, "optional sibling node id to place before")
	fs.StringVar(&name, "name", "", "optional rename while moving")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
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

	nodePath = strings.TrimSpace(nodePath)
	if nodeID > 0 && nodePath != "" {
		return errors.New("`--node-id` and `--node-path` cannot be used together")
	}
	if nodeID == 0 && nodePath == "" {
		return errors.New("one of `--node-id` or `--node-path` is required")
	}

	newParentPath = strings.TrimSpace(newParentPath)
	if newParentID > 0 && newParentPath != "" {
		return errors.New("`--new-parent-id` and `--new-parent-path` cannot be used together")
	}
	if newParentID == 0 && newParentPath == "" {
		return errors.New("one of `--new-parent-id` or `--new-parent-path` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	if nodePath != "" {
		_, node, err := resolveNodeByPath(context.Background(), client, libraryID, nodePath)
		if err != nil {
			return err
		}
		nodeID = node.ID
	}

	if newParentPath != "" {
		_, parentNode, err := resolveNodeByPath(context.Background(), client, libraryID, newParentPath)
		if err != nil {
			return err
		}
		if parentNode.Type != "dir" {
			return fmt.Errorf("`--new-parent-path` must resolve to a directory, got type=%s", parentNode.Type)
		}
		newParentID = parentNode.ID
	}

	if err := client.MoveNodesBatch(context.Background(), MoveNodesBatchRequest{
		NewParentID:  newParentID,
		BeforeNodeID: beforeNodeID,
		LibraryID:    libraryID,
		Items: []MoveNodeBatchItemRequest{
			{
				NodeID: nodeID,
				Name:   strings.TrimSpace(name),
			},
		},
	}, dryRun); err != nil {
		return err
	}

	result := map[string]any{
		"dryRun":       dryRun,
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
	if dryRun {
		a.printf("dry-run: move request validated: id=%d library=%d new_parent=%d before=%d\n", nodeID, libraryID, newParentID, beforeNodeID)
		return nil
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
		pathValue string
		dryRun    bool
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Uint64Var(&nodeID, "node-id", 0, "target node id (required)")
	fs.StringVar(&pathValue, "path", "", "target node path from root, e.g. /docs/a.txt")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
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

	pathValue = strings.TrimSpace(pathValue)
	if nodeID > 0 && pathValue != "" {
		return errors.New("`--node-id` and `--path` cannot be used together")
	}
	if nodeID == 0 && pathValue == "" {
		return errors.New("one of `--node-id` or `--path` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	if pathValue != "" {
		_, node, err := resolveNodeByPath(context.Background(), client, libraryID, pathValue)
		if err != nil {
			return err
		}
		nodeID = node.ID
	}

	ok, err := client.DeleteNodeTree(context.Background(), nodeID, libraryID, dryRun)
	if err != nil {
		return err
	}

	result := map[string]any{
		"dryRun":    dryRun,
		"ok":        ok,
		"nodeId":    nodeID,
		"libraryId": libraryID,
	}
	if jsonOut {
		return a.printJSON(result)
	}
	if dryRun {
		if ok {
			a.printf("dry-run: recycle request validated: id=%d library=%d\n", nodeID, libraryID)
			return nil
		}
		a.printf("dry-run: recycle request validated but no change: id=%d library=%d\n", nodeID, libraryID)
		return nil
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
		dryRun    bool
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Uint64Var(&nodeID, "node-id", 0, "target node id in recycle bin (required)")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
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

	ok, err := client.RestoreNodeTree(context.Background(), nodeID, libraryID, dryRun)
	if err != nil {
		return err
	}

	result := map[string]any{
		"dryRun":    dryRun,
		"ok":        ok,
		"nodeId":    nodeID,
		"libraryId": libraryID,
	}
	if jsonOut {
		return a.printJSON(result)
	}
	if dryRun {
		if ok {
			a.printf("dry-run: restore request validated: id=%d library=%d\n", nodeID, libraryID)
			return nil
		}
		a.printf("dry-run: restore request validated but no change: id=%d library=%d\n", nodeID, libraryID)
		return nil
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
		dryRun    bool
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Uint64Var(&nodeID, "node-id", 0, "target node id in recycle bin (required)")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
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

	ok, err := client.HardDeleteNodeTree(context.Background(), nodeID, libraryID, dryRun)
	if err != nil {
		return err
	}

	result := map[string]any{
		"dryRun":    dryRun,
		"ok":        ok,
		"nodeId":    nodeID,
		"libraryId": libraryID,
	}
	if jsonOut {
		return a.printJSON(result)
	}
	if dryRun {
		if ok {
			a.printf("dry-run: hard-delete request validated: id=%d library=%d\n", nodeID, libraryID)
			return nil
		}
		a.printf("dry-run: hard-delete request validated but no change: id=%d library=%d\n", nodeID, libraryID)
		return nil
	}
	if ok {
		a.printf("hard deleted node tree from recycle bin: id=%d library=%d\n", nodeID, libraryID)
		return nil
	}
	a.printf("node tree was not hard deleted: id=%d library=%d\n", nodeID, libraryID)
	return nil
}

func (a *App) runFSArchiveBatchSetBuiltInType(args []string) error {
	fs := a.newFlagSet("fs archive batch-set-built-in-type")

	var (
		baseURL string
		nodeID  uint64
		dryRun  bool
		jsonOut bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&nodeID, "node-id", 0, "archive directory node id (required, >0)")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit changes")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if nodeID == 0 {
		return errors.New("`--node-id` is required and must be greater than 0")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	result, err := client.BatchSetArchiveChildrenBuiltInType(context.Background(), nodeID, dryRun)
	if err != nil {
		return err
	}

	if jsonOut {
		return a.printJSON(map[string]any{
			"dryRun":        dryRun,
			"nodeId":        result.NodeID,
			"libraryId":     result.LibraryID,
			"builtInType":   result.BuiltInType,
			"totalChildren": result.TotalChildren,
			"dirChildren":   result.DirChildren,
			"updatedCount":  result.UpdatedCount,
		})
	}

	if dryRun {
		a.printf(
			"dry-run: archive batch set validated: nodeId=%d updatedCount=%d dirChildren=%d builtInType=%s\n",
			result.NodeID,
			result.UpdatedCount,
			result.DirChildren,
			result.BuiltInType,
		)
		return nil
	}

	a.printf(
		"archive batch set completed: nodeId=%d updatedCount=%d dirChildren=%d builtInType=%s\n",
		result.NodeID,
		result.UpdatedCount,
		result.DirChildren,
		result.BuiltInType,
	)
	return nil
}

func (a *App) runFSPathResolve(args []string) error {
	fs := a.newFlagSet("fs path resolve")

	var (
		baseURL   string
		libraryID uint64
		pathValue string
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.StringVar(&pathValue, "path", "", "node path, e.g. /docs/ch1 (required)")
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

	canonicalPath, resolvedNode, err := resolveNodeByPath(context.Background(), client, libraryID, pathValue)
	if err != nil {
		return err
	}

	result := map[string]any{
		"libraryId": libraryID,
		"path":      canonicalPath,
		"nodeId":    resolvedNode.ID,
		"node":      resolvedNode,
	}
	if jsonOut {
		return a.printJSON(result)
	}

	a.printf(
		"resolved path: path=%s node_id=%d type=%s name=%s parent=%d library=%d\n",
		canonicalPath,
		resolvedNode.ID,
		resolvedNode.Type,
		resolvedNode.Name,
		resolvedNode.ParentID,
		resolvedNode.LibraryID,
	)
	return nil
}

func findChildNodeByName(children []Node, name string) (Node, error) {
	var matched []Node
	for _, child := range children {
		if child.Name == name {
			matched = append(matched, child)
		}
	}

	if len(matched) == 0 {
		return Node{}, errors.New("node not found")
	}
	if len(matched) > 1 {
		return Node{}, fmt.Errorf("ambiguous node name %q (%d matches)", name, len(matched))
	}
	return matched[0], nil
}

func resolveNodeByPath(ctx context.Context, client *Client, libraryID uint64, pathValue string) (string, Node, error) {
	canonicalPath, segments, err := normalizeNodePath(pathValue)
	if err != nil {
		return "", Node{}, err
	}

	rootID, err := client.GetLibraryRootNodeID(ctx, libraryID)
	if err != nil {
		return "", Node{}, err
	}

	currentID := rootID
	resolvedNode := Node{
		ID:        rootID,
		Type:      "dir",
		ParentID:  0,
		LibraryID: libraryID,
		Name:      "/",
	}

	for idx, segment := range segments {
		children, err := client.ListChildren(ctx, currentID, libraryID)
		if err != nil {
			return "", Node{}, err
		}

		nextNode, err := findChildNodeByName(children, segment)
		if err != nil {
			return "", Node{}, fmt.Errorf("resolve %q failed at segment %q: %w", canonicalPath, segment, err)
		}
		if idx < len(segments)-1 && nextNode.Type != "dir" {
			return "", Node{}, fmt.Errorf("resolve %q failed at segment %q: segment is not a directory", canonicalPath, segment)
		}

		currentID = nextNode.ID
		resolvedNode = nextNode
	}

	return canonicalPath, resolvedNode, nil
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
