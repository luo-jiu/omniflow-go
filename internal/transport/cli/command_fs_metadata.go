package cli

import (
	"context"
	"errors"
	"net/http"
	domainnode "omniflow-go/internal/domain/node"
	"strings"
)

// MetadataQueryRequest 使用正式元数据 API，不通过 CLI 直接读取仓储。
type MetadataQueryRequest struct {
	domainnode.MetadataQuery
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// BrowseMetadata 读取一页元数据，保留服务端分页语义。
func (c *Client) BrowseMetadata(ctx context.Context, req MetadataQueryRequest) (domainnode.MetadataPage, error) {
	var out domainnode.MetadataPage
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/nodes/metadata/query", nil, req, true, &out)
	return out, err
}

func (a *App) runFSBrowse(args []string) error {
	fs := a.newFlagSet("fs browse")
	var baseURL, nodeType, tagIDs string
	var jsonOut bool
	var req MetadataQueryRequest
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&req.LibraryID, "library-id", 0, "library id (required)")
	fs.StringVar(&req.Mode, "mode", "children", "root, children, search or node")
	fs.Uint64Var(&req.NodeID, "node-id", 0, "node id for node mode")
	fs.Uint64Var(&req.ParentID, "parent-id", 0, "parent directory id (default root)")
	fs.Uint64Var(&req.AncestorID, "ancestor-id", 0, "search subtree directory id")
	fs.StringVar(&req.Keyword, "keyword", "", "literal name substring")
	fs.StringVar(&nodeType, "node-type", "", "dir or file")
	fs.StringVar(&tagIDs, "tag-ids", "", "comma-separated tag ids")
	fs.StringVar(&req.TagMatchMode, "tag-match-mode", "ANY", "ANY or ALL")
	fs.StringVar(&req.Cursor, "cursor", "", "cursor from the previous page")
	fs.IntVar(&req.Limit, "limit", 50, "page size, 1..100")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if req.LibraryID == 0 {
		return errors.New("`--library-id` is required")
	}
	if req.Limit < 1 || req.Limit > 100 {
		return errors.New("`--limit` must be 1..100")
	}
	if req.Mode != "root" && req.Mode != "children" && req.Mode != "search" && req.Mode != "node" {
		return errors.New("invalid `--mode`")
	}
	if nodeType != "" && nodeType != "dir" && nodeType != "file" {
		return errors.New("invalid `--node-type`")
	}
	req.NodeType = domainnode.Type(nodeType)
	req.Keyword = strings.TrimSpace(req.Keyword)
	var err error
	req.TagIDs, err = parseUint64CSV(tagIDs)
	if err != nil {
		return err
	}
	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}
	page, err := client.BrowseMetadata(context.Background(), req)
	if err != nil {
		return err
	}
	if jsonOut {
		return a.printJSON(page)
	}
	if req.Mode == "root" {
		a.printf("%d\t%s\n", page.Root.ID, page.Root.Path)
	}
	for _, node := range page.Entries {
		a.printf("%d\t%s\t%s\n", node.ID, node.Type, node.Path)
	}
	if page.HasMore {
		a.printf("next cursor: %s\n", page.NextCursor)
	}
	return nil
}
