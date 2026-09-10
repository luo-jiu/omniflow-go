package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"omniflow-go/internal/actor"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/repository"
	"omniflow-go/internal/storage"
)

const maxCopyNodes = 1000
const maxCopyBytes int64 = 1 << 30

// CopyNodeCommand 在同一资料库创建独立副本，Name 是完整文件名。
type CopyNodeCommand struct {
	Actor                       actor.Actor
	NodeID, LibraryID, ParentID uint64
	Name                        string
	Recursive                   bool
	ConflictPolicy              NodeNameConflictPolicy
	DryRun                      bool
}

// CopyNodeResult 返回根副本及需要刷新的父目录；预演节点 ID 不可用于后续操作。
type CopyNodeResult struct {
	Node              domainnode.Node `json:"node"`
	CopiedCount       int             `json:"copiedCount"`
	AffectedParentIDs []uint64        `json:"affectedParentIds"`
	DryRun            bool            `json:"dryRun"`
}

type nodeCopyItem struct {
	node  domainnode.Node
	store storage.ObjectStorage
	key   string
}

// Copy 复制节点及文件对象，不共享源文件的可变存储对象。
func (u *NodeUseCase) Copy(ctx context.Context, cmd CopyNodeCommand) (CopyNodeResult, error) {
	if cmd.NodeID == 0 || cmd.LibraryID == 0 || cmd.ParentID == 0 {
		return CopyNodeResult{}, fmt.Errorf("%w: nodeId, libraryId and parentId are required", ErrInvalidArgument)
	}
	if cmd.ConflictPolicy != "" && cmd.ConflictPolicy != NodeNameConflictError && cmd.ConflictPolicy != NodeNameConflictAutoRename {
		return CopyNodeResult{}, fmt.Errorf("%w: conflictPolicy only supports error or auto_rename", ErrInvalidArgument)
	}
	if err := u.ensureNodesConfigured(); err != nil {
		return CopyNodeResult{}, err
	}
	if u.tx == nil {
		return CopyNodeResult{}, fmt.Errorf("%w: copy requires transaction manager", ErrInvalidArgument)
	}
	if err := u.AuthorizeMutation(ctx, cmd.Actor, cmd.LibraryID); err != nil {
		return CopyNodeResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var result CopyNodeResult
	var uploaded []nodeCopyItem
	callbackComplete := false
	err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		items, err := u.planNodeCopy(txCtx, cmd)
		if err != nil {
			return err
		}
		// 先用真实创建链路验证所有节点，再复制对象；任何失败均回滚整棵子树。
		ids := map[uint64]uint64{items[0].node.ParentID: cmd.ParentID}
		result = CopyNodeResult{CopiedCount: len(items), AffectedParentIDs: []uint64{cmd.ParentID}, DryRun: cmd.DryRun}
		for i, item := range items {
			n := item.node
			parentID := ids[n.ParentID]
			if i == 0 {
				parentID = cmd.ParentID
			}
			policy := NodeNameConflictError
			if i == 0 {
				policy = cmd.ConflictPolicy
			}
			created, err := u.createInExistingTransaction(txCtx, CreateNodeCommand{
				Actor: cmd.Actor, LibraryID: cmd.LibraryID, ParentID: parentID,
				Name: n.Name, Ext: n.Ext, Type: n.Type, MIMEType: n.MIMEType, FileSize: n.FileSize,
				StorageKey: item.key, StorageProvider: n.StorageProvider, StorageBucket: n.StorageBucket,
				ConflictPolicy: policy, DryRun: cmd.DryRun,
			})
			if err != nil {
				return err
			}
			ids[n.ID] = created.ID
			if i == 0 {
				result.Node = created
			}
		}
		if !cmd.DryRun {
			for _, item := range items {
				if item.store == nil {
					continue
				}
				// Upload 失败也可能留下对象，提前登记清理目标。
				uploaded = append(uploaded, item)
				if err := copyNodeObject(txCtx, item); err != nil {
					return err
				}
			}
		}
		callbackComplete = true
		return nil
	})
	if err != nil {
		if callbackComplete && !cmd.DryRun {
			slog.ErrorContext(ctx, "node.copy.commit_unknown", "node_id", result.Node.ID, "library_id", cmd.LibraryID)
			return CopyNodeResult{}, fmt.Errorf("copy commit outcome unknown; inspect destination before retrying (nodeId=%d): %w", result.Node.ID, err)
		}
		cleanupNodeCopies(ctx, uploaded)
		return CopyNodeResult{}, err
	}
	if cmd.DryRun {
		result.Node.ID = 0
		result.Node.StorageKey = ""
	}
	_ = u.writeAudit(ctx, cmd.Actor, "node.copy", true, map[string]any{
		"node_id": cmd.NodeID, "copied_node_id": result.Node.ID, "library_id": cmd.LibraryID,
		"parent_id": cmd.ParentID, "copied_count": result.CopiedCount, "dry_run": cmd.DryRun, "mode": resolveMutationMode(cmd.DryRun),
	})
	slog.InfoContext(ctx, "node.copy.completed", "node_id", cmd.NodeID, "copied_count", result.CopiedCount, "dry_run", cmd.DryRun)
	return result, nil
}

func (u *NodeUseCase) planNodeCopy(ctx context.Context, cmd CopyNodeCommand) ([]nodeCopyItem, error) {
	source, err := u.copyNodeView(ctx, cmd.NodeID, cmd.LibraryID)
	if err != nil {
		return nil, err
	}
	if source.ParentID == 0 {
		return nil, fmt.Errorf("%w: library root cannot be copied", ErrInvalidArgument)
	}
	if source.Type == domainnode.TypeDirectory && !cmd.Recursive {
		return nil, fmt.Errorf("%w: directory copy requires recursive=true", ErrInvalidArgument)
	}
	parent, err := u.copyNodeView(ctx, cmd.ParentID, cmd.LibraryID)
	if err != nil {
		return nil, err
	}
	if parent.Type != domainnode.TypeDirectory {
		return nil, fmt.Errorf("%w: parent must be a directory", ErrInvalidArgument)
	}
	ancestors, err := u.nodes.ListAncestors(ctx, cmd.ParentID, cmd.LibraryID)
	if err != nil {
		return nil, err
	}
	if cmd.ParentID == cmd.NodeID {
		return nil, fmt.Errorf("%w: cannot copy into self", ErrInvalidArgument)
	}
	for _, n := range ancestors {
		if n.ID == cmd.NodeID {
			return nil, fmt.Errorf("%w: cannot copy into descendant", ErrInvalidArgument)
		}
	}
	items := []nodeCopyItem{{node: source}}
	seen := map[uint64]bool{source.ID: true}
	var total int64
	for i := 0; i < len(items); i++ {
		item := &items[i]
		n := item.node
		if n.Type == domainnode.TypeDirectory {
			children, err := u.nodes.ListDirectChildrenLimited(ctx, n.ID, cmd.LibraryID, maxCopyNodes-len(items)+1)
			if err != nil {
				return nil, err
			}
			if len(items)+len(children) > maxCopyNodes {
				return nil, fmt.Errorf("%w: copy exceeds 1000 nodes", ErrInvalidArgument)
			}
			for _, child := range children {
				if seen[child.ID] {
					return nil, fmt.Errorf("%w: cyclic node tree", ErrInvalidArgument)
				}
				seen[child.ID] = true
				items = append(items, nodeCopyItem{node: child})
			}
		} else if n.Type == domainnode.TypeFile {
			if u.registry == nil || n.StorageKey == "" {
				return nil, fmt.Errorf("%w: source file has no readable storage binding", ErrInvalidArgument)
			}
			store, err := u.registry.Get(n.StorageProvider)
			if err != nil {
				return nil, err
			}
			if n.StorageBucket != store.Bucket() {
				return nil, fmt.Errorf("%w: source bucket differs from configured provider", ErrInvalidArgument)
			}
			info, err := store.StatObject(ctx, n.StorageKey)
			if err != nil {
				return nil, fmt.Errorf("source node %d unavailable: %w", n.ID, err)
			}
			if info.Size < 0 || info.Size != n.FileSize {
				return nil, fmt.Errorf("%w: source size changed", ErrConflict)
			}
			if info.Size > maxCopyBytes-total {
				return nil, fmt.Errorf("%w: copy exceeds 1 GiB", ErrInvalidArgument)
			}
			total += info.Size
			item.store = store
			item.key = fmt.Sprintf("libraries/%d/%s", cmd.LibraryID, uuid.NewString())
		} else {
			return nil, fmt.Errorf("%w: unsupported source node type", ErrInvalidArgument)
		}
	}
	if cmd.Name != "" {
		name, ext, err := copyNodeName(cmd.Name, source.Type)
		if err != nil {
			return nil, err
		}
		items[0].node.Name, items[0].node.Ext = name, ext
	}
	return items, nil
}

func copyNodeName(full string, kind domainnode.Type) (string, string, error) {
	if strings.TrimSpace(full) != full || full == "" || full == "." || full == ".." || strings.ContainsAny(full, "/\\\x00\r\n") || utf8.RuneCountInString(full) > 255 {
		return "", "", fmt.Errorf("%w: name must be a valid complete filename", ErrInvalidArgument)
	}
	name, ext := full, ""
	if kind == domainnode.TypeFile {
		suffix := path.Ext(full)
		if suffix == "." {
			return "", "", fmt.Errorf("%w: filename cannot end with a dot", ErrInvalidArgument)
		}
		if suffix != full {
			name, ext = strings.TrimSuffix(full, suffix), strings.TrimPrefix(suffix, ".")
		}
	}
	if utf8.RuneCountInString(ext) > maxNodeExtLength {
		return "", "", fmt.Errorf("%w: ext is too long", ErrInvalidArgument)
	}
	return name, ext, nil
}

func (u *NodeUseCase) copyNodeView(ctx context.Context, id, libraryID uint64) (domainnode.Node, error) {
	n, err := u.nodes.FindViewForCopy(ctx, id, libraryID)
	if errors.Is(err, repository.ErrNotFound) {
		return domainnode.Node{}, ErrNotFound
	}
	return n, err
}

func copyNodeObject(ctx context.Context, item nodeCopyItem) error {
	r, info, err := item.store.GetObject(ctx, item.node.StorageKey)
	if err != nil {
		return err
	}
	defer r.Close()
	if info.Size != item.node.FileSize {
		return fmt.Errorf("%w: source size changed", ErrConflict)
	}
	limited := &io.LimitedReader{R: r, N: info.Size}
	if err := item.store.Upload(ctx, item.key, limited, info.Size, item.node.MIMEType); err != nil {
		return err
	}
	if limited.N != 0 {
		return fmt.Errorf("%w: source stream ended before declared size", ErrConflict)
	}
	return nil
}

func cleanupNodeCopies(ctx context.Context, items []nodeCopyItem) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	for _, item := range items {
		if err := item.store.Delete(cleanupCtx, item.key); err != nil {
			slog.WarnContext(cleanupCtx, "node.copy.cleanup_failed", "object_key", item.key, "error", err)
		}
	}
}
