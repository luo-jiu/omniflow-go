package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// NodeTagDeltaRequest 与节点增删标签 HTTP 契约一致。
type NodeTagDeltaRequest struct {
	LibraryID    uint64   `json:"libraryId"`
	AddTagIDs    []uint64 `json:"addTagIds,omitempty"`
	RemoveTagIDs []uint64 `json:"removeTagIds,omitempty"`
}

// UpdateNodeTags 发出单次原子增删请求，不自动重试结果未知的写入。
func (c *Client) UpdateNodeTags(ctx context.Context, nodeID uint64, req NodeTagDeltaRequest, dryRun bool) (map[string]any, error) {
	var out map[string]any
	err := c.doJSON(ctx, http.MethodPatch, fmt.Sprintf("/api/v1/nodes/%d/tags", nodeID), withDryRunQuery(nil, dryRun), req, true, &out)
	return out, err
}

// ReadNodeTags 读取正式节点标签关系。
func (c *Client) ReadNodeTags(ctx context.Context, nodeID, libraryID uint64) (map[string]any, error) {
	var out map[string]any
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/nodes/%d/tags", nodeID), url.Values{"libraryId": {strconv.FormatUint(libraryID, 10)}}, nil, true, &out)
	return out, err
}

func parseTagDeltaIDs(raw string) ([]uint64, error) {
	ids := []uint64{}
	if raw == "" {
		return ids, nil
	}
	for _, part := range strings.Split(raw, ",") {
		id, err := strconv.ParseUint(strings.TrimSpace(part), 10, 63)
		if err != nil || id == 0 {
			return nil, errors.New("tag IDs must be positive integers")
		}
		ids = append(ids, id)
	}
	if len(ids) > 100 {
		return nil, errors.New("at most 100 tag IDs per list")
	}
	return ids, nil
}

func (a *App) runFSTags(args []string) error       { return a.runNodeTags(args, false) }
func (a *App) runFSTagsUpdate(args []string) error { return a.runNodeTags(args, true) }

func (a *App) runNodeTags(args []string, write bool) error {
	fs := a.newFlagSet("fs tags")
	var nodeID, libraryID uint64
	var baseURL, add, remove string
	var dryRun, jsonOut bool
	fs.Uint64Var(&nodeID, "node-id", 0, "node id (required)")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.StringVar(&baseURL, "base-url", "", "API base URL")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if write {
		fs.StringVar(&add, "add-tag-ids", "", "comma-separated tag IDs to add")
		fs.StringVar(&remove, "remove-tag-ids", "", "comma-separated tag IDs to remove")
		fs.BoolVar(&dryRun, "dry-run", false, "validate and roll back")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if nodeID == 0 || libraryID == 0 {
		return errors.New("--node-id and --library-id are required")
	}
	addIDs, err := parseTagDeltaIDs(add)
	if err != nil {
		return err
	}
	removeIDs, err := parseTagDeltaIDs(remove)
	if err != nil {
		return err
	}
	if write && len(addIDs)+len(removeIDs) == 0 {
		return errors.New("provide --add-tag-ids or --remove-tag-ids")
	}
	seen := map[uint64]bool{}
	for _, id := range append(append([]uint64{}, addIDs...), removeIDs...) {
		if seen[id] {
			return errors.New("tag IDs must be unique and disjoint")
		}
		seen[id] = true
	}
	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}
	var out map[string]any
	if write {
		out, err = client.UpdateNodeTags(context.Background(), nodeID, NodeTagDeltaRequest{LibraryID: libraryID, AddTagIDs: addIDs, RemoveTagIDs: removeIDs}, dryRun)
	} else {
		out, err = client.ReadNodeTags(context.Background(), nodeID, libraryID)
	}
	if err != nil {
		return err
	}
	if jsonOut {
		return a.printJSON(out)
	}
	if dryRun {
		if result, ok := out["result"].(map[string]any); ok {
			out = result
		}
	}
	a.printf("node tags: node=%d tags=%v dry_run=%t\n", nodeID, out["tagIds"], dryRun)
	return nil
}
