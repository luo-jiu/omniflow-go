package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// CopyNodeRequest 描述同库独立复制请求。
type CopyNodeRequest struct {
	LibraryID      uint64 `json:"libraryId"`
	ParentID       uint64 `json:"parentId"`
	Name           string `json:"name,omitempty"`
	Recursive      bool   `json:"recursive"`
	ConflictPolicy string `json:"conflictPolicy,omitempty"`
}

// CopyNode 通过 HTTP 复制节点，不自动重试结果不明的写请求。
func (c *Client) CopyNode(ctx context.Context, nodeID uint64, req CopyNodeRequest, dryRun bool) (map[string]any, error) {
	var out map[string]any
	copyClient := *c
	httpClient := *c.httpClient
	httpClient.Timeout = 130 * time.Second
	copyClient.httpClient = &httpClient
	err := copyClient.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/v1/nodes/%d/copy", nodeID), withDryRunQuery(nil, dryRun), req, true, &out)
	return out, err
}

func (a *App) runFSCopy(args []string) error {
	fs := a.newFlagSet("fs cp")
	var baseURL string
	var nodeID uint64
	var req CopyNodeRequest
	var dryRun, jsonOut bool
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&nodeID, "node-id", 0, "source node id (required)")
	fs.Uint64Var(&req.LibraryID, "library-id", 0, "library id (required)")
	fs.Uint64Var(&req.ParentID, "parent-id", 0, "destination directory id (required)")
	fs.StringVar(&req.Name, "name", "", "optional complete filename")
	fs.StringVar(&req.ConflictPolicy, "conflict-policy", "error", "error or auto_rename")
	fs.BoolVar(&req.Recursive, "recursive", false, "explicitly copy directory contents")
	fs.BoolVar(&dryRun, "dry-run", false, "validate without committing or writing objects")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if nodeID == 0 || req.LibraryID == 0 || req.ParentID == 0 {
		return errors.New("--node-id, --library-id and --parent-id are required")
	}
	if req.ConflictPolicy != "error" && req.ConflictPolicy != "auto_rename" {
		return errors.New("--conflict-policy must be error or auto_rename")
	}
	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}
	out, err := client.CopyNode(context.Background(), nodeID, req, dryRun)
	if err != nil {
		return err
	}
	if jsonOut {
		return a.printJSON(out)
	}
	a.printf("copy: dry_run=%t copied_count=%v node=%v\n", dryRun, out["copiedCount"], out["node"])
	return nil
}
