package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"omniflow-go/internal/actor"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/repository"
	"sort"
	"strings"
	"time"
)

// BrowseNodeMetadataQuery 包含查询与分页参数，Actor 始终由服务端提供。
type BrowseNodeMetadataQuery struct {
	Actor  actor.Actor
	Query  domainnode.MetadataQuery
	Cursor string
	Limit  int
}

type metadataCursor struct {
	After uint64 `json:"after"`
	Scope string `json:"scope"`
}

func normalizeMetadataQuery(q domainnode.MetadataQuery) (domainnode.MetadataQuery, error) {
	if q.LibraryID == 0 || q.LibraryID > math.MaxInt64 || q.ParentID > math.MaxInt64 || q.AncestorID > math.MaxInt64 || q.NodeID > math.MaxInt64 {
		return q, fmt.Errorf("%w: invalid node scope", ErrInvalidArgument)
	}
	if q.Mode == "" {
		q.Mode = "children"
	}
	if q.Mode != "children" && q.Mode != "search" && q.Mode != "root" && q.Mode != "node" {
		return q, fmt.Errorf("%w: invalid metadata mode", ErrInvalidArgument)
	}
	if q.NodeType != "" && q.NodeType != domainnode.TypeDirectory && q.NodeType != domainnode.TypeFile {
		return q, fmt.Errorf("%w: invalid node type", ErrInvalidArgument)
	}
	q.Keyword = strings.TrimSpace(q.Keyword)
	q.TagMatchMode = strings.ToUpper(strings.TrimSpace(q.TagMatchMode))
	if q.TagMatchMode == "" {
		q.TagMatchMode = "ANY"
	}
	if (q.TagMatchMode != "ANY" && q.TagMatchMode != "ALL") || len(q.Keyword) > 512 || len(q.Names) > 2 || len(q.TagIDs) > 50 {
		return q, fmt.Errorf("%w: invalid metadata filters", ErrInvalidArgument)
	}
	for _, name := range q.Names {
		if name == "" || len(name) > 512 {
			return q, fmt.Errorf("%w: invalid exact node name", ErrInvalidArgument)
		}
	}
	for _, id := range q.TagIDs {
		if id == 0 || id > math.MaxInt64 {
			return q, fmt.Errorf("%w: invalid tag id", ErrInvalidArgument)
		}
	}
	q.TagIDs = normalizePositiveUint64List(q.TagIDs)
	sort.Slice(q.TagIDs, func(i, j int) bool { return q.TagIDs[i] < q.TagIDs[j] })
	q.Names = append([]string(nil), q.Names...)
	sort.Strings(q.Names)
	if (q.Mode != "children" && q.ParentID != 0) || (q.Mode != "search" && q.AncestorID != 0) {
		return q, fmt.Errorf("%w: incompatible metadata scope", ErrInvalidArgument)
	}
	if (q.Mode == "node") != (q.NodeID > 0) {
		return q, fmt.Errorf("%w: node mode requires only nodeId", ErrInvalidArgument)
	}
	if (q.Mode == "root" || q.Mode == "node") && (q.Keyword != "" || q.NodeType != "" || len(q.Names) > 0 || len(q.TagIDs) > 0) {
		return q, fmt.Errorf("%w: root query does not accept filters", ErrInvalidArgument)
	}
	return q, nil
}

// BrowseNodeMetadata 查询不会创建根节点、修复目录或访问 MinIO。
func (u *NodeUseCase) BrowseNodeMetadata(ctx context.Context, input BrowseNodeMetadataQuery) (domainnode.MetadataPage, error) {
	if err := u.ensureNodesConfigured(); err != nil {
		return domainnode.MetadataPage{}, err
	}
	page, err := browseNodeMetadata(ctx, input, u.nodes, u.AuthorizeRead)
	if errors.Is(err, repository.ErrNotFound) {
		return page, ErrNotFound
	}
	return page, err
}

func browseNodeMetadata(ctx context.Context, input BrowseNodeMetadataQuery, reader domainnode.MetadataReader,
	authorize func(context.Context, actor.Actor, uint64) error) (domainnode.MetadataPage, error) {
	page := domainnode.MetadataPage{Entries: []domainnode.MetadataEntry{}}
	q, err := normalizeMetadataQuery(input.Query)
	if err != nil {
		return page, err
	}
	if input.Limit == 0 {
		input.Limit = 50
	}
	if input.Limit < 1 || input.Limit > 100 || len(input.Cursor) > 256 || ((q.Mode == "root" || q.Mode == "node") && input.Cursor != "") {
		return page, fmt.Errorf("%w: invalid metadata pagination", ErrInvalidArgument)
	}
	if err := authorize(ctx, input.Actor, q.LibraryID); err != nil {
		return page, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	root, err := reader.ReadMetadataRoot(ctx, q.LibraryID)
	if err != nil {
		return page, err
	}
	if root.LibraryID != q.LibraryID || root.Type != domainnode.TypeDirectory {
		return page, ErrNotFound
	}
	page.Root = root
	if q.Mode == "root" {
		return page, nil
	}
	if q.Mode == "node" && q.NodeID == root.ID {
		page.Entries = []domainnode.MetadataEntry{root}
		return page, nil
	}
	if q.Mode == "children" && q.ParentID == 0 {
		q.ParentID = root.ID
	}
	for _, id := range []uint64{q.ParentID, q.AncestorID} {
		if id == 0 || id == root.ID {
			continue
		}
		node, err := reader.ReadMetadataNode(ctx, id, q.LibraryID)
		if err != nil {
			return page, err
		}
		if node.ID != id || node.LibraryID != q.LibraryID || node.Type != domainnode.TypeDirectory {
			return page, ErrNotFound
		}
	}
	serialized, _ := json.Marshal(q)
	hash := sha256.Sum256(serialized)
	scope := hex.EncodeToString(hash[:])
	var cursor metadataCursor
	if input.Cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(input.Cursor)
		if err != nil || json.Unmarshal(decoded, &cursor) != nil || cursor.Scope != scope || cursor.After == 0 || cursor.After > math.MaxInt64 {
			return page, fmt.Errorf("%w: cursor does not match query", ErrInvalidArgument)
		}
	}
	rows, err := reader.QueryMetadata(ctx, q, root.ID, cursor.After, input.Limit+1)
	if err != nil {
		return page, err
	}
	page.HasMore = len(rows) > input.Limit
	if page.HasMore {
		rows = rows[:input.Limit]
	}
	page.Entries = append(page.Entries, rows...)
	if page.HasMore {
		data, _ := json.Marshal(metadataCursor{After: rows[len(rows)-1].ID, Scope: scope})
		page.NextCursor = base64.RawURLEncoding.EncodeToString(data)
	}
	slog.DebugContext(ctx, "node.metadata.listed", "library_id", q.LibraryID, "mode", q.Mode, "result_count", len(rows), "has_more", page.HasMore)
	return page, nil
}
