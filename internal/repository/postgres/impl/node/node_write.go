package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domainnode "omniflow-go/internal/domain/node"
	pgmodel "omniflow-go/internal/repository/postgres/model"

	"gorm.io/gorm"
)

type CreateNodeInput struct {
	Name            string
	Type            domainnode.Type
	ParentID        uint64
	LibraryID       uint64
	Ext             string
	MIMEType        string
	FileSize        int64
	StorageKey      string
	BuiltInType     string
	ArchiveMode     bool
	StorageProvider string
	StorageBucket   string
}

type MoveNodeInput struct {
	LibraryID    uint64
	NodeID       uint64
	NewParentID  uint64
	BeforeNodeID uint64
	Name         string
	UpdatedAt    time.Time
}

// CreateNode 创建节点并在文件类型时补齐对象存储关联。
func (r *NodeRepository) CreateNode(ctx context.Context, input CreateNodeInput) (domainnode.Node, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domainnode.Node{}, fmt.Errorf("%w: node name is required", ErrInvalidState)
	}

	if input.ParentID > 0 {
		parent, err := r.findNodeModel(ctx, input.ParentID, input.LibraryID)
		if err != nil {
			return domainnode.Node{}, err
		}
		if parent.NodeType != nodeTypeDirectory {
			return domainnode.Node{}, fmt.Errorf("%w: parent node must be directory", ErrInvalidState)
		}
	}

	duplicate, err := r.hasDuplicateName(ctx, name, input.ParentID, input.LibraryID, 0)
	if err != nil {
		return domainnode.Node{}, err
	}
	if duplicate {
		return domainnode.Node{}, ErrConflict
	}

	maxSort, err := r.maxSortOrder(ctx, input.LibraryID, input.ParentID)
	if err != nil {
		return domainnode.Node{}, err
	}

	var ext *string
	if input.Type == domainnode.TypeFile {
		trimmedExt := strings.TrimSpace(input.Ext)
		if trimmedExt != "" {
			ext = &trimmedExt
		}
		if strings.TrimSpace(input.StorageKey) == "" {
			return domainnode.Node{}, fmt.Errorf("%w: storage key is required for file node", ErrInvalidState)
		}
		if input.FileSize < 0 {
			return domainnode.Node{}, fmt.Errorf("%w: file size must be >= 0", ErrInvalidState)
		}
	}

	builtInType := strings.TrimSpace(input.BuiltInType)
	if builtInType == "" {
		builtInType = "DEF"
	}

	row := &pgmodel.Node{
		Name:        name,
		Ext:         ext,
		BuiltInType: builtInType,
		NodeType:    nodeTypeCode(input.Type),
		ArchiveMode: input.ArchiveMode,
		ViewMeta:    "{}",
		SortOrder:   int32(maxSort + 15),
		ParentID:    normalizePGParentID(input.ParentID),
		LibraryID:   toPGInt64(input.LibraryID),
	}
	q := r.query(ctx)
	if err := q.Node.WithContext(ctx).Create(row); err != nil {
		return domainnode.Node{}, err
	}

	if input.Type == domainnode.TypeFile {
		storageRow := &pgmodel.StorageObject{
			LibraryID:     toPGInt64(input.LibraryID),
			Provider:      strings.TrimSpace(input.StorageProvider),
			Bucket:        strings.TrimSpace(input.StorageBucket),
			ObjectKey:     strings.TrimSpace(input.StorageKey),
			ContentLength: input.FileSize,
			ContentType:   nullableString(strings.TrimSpace(input.MIMEType)),
			Extra:         "{}",
		}
		if err := q.StorageObject.WithContext(ctx).Create(storageRow); err != nil {
			return domainnode.Node{}, err
		}

		fileRow := &pgmodel.NodeFile{
			FileID:          row.ID,
			LibraryID:       toPGInt64(input.LibraryID),
			StorageObjectID: storageRow.ID,
			MimeType:        nullableString(strings.TrimSpace(input.MIMEType)),
			FileSize:        input.FileSize,
		}
		if err := q.NodeFile.WithContext(ctx).Create(fileRow); err != nil {
			return domainnode.Node{}, err
		}
	}

	return r.FindViewByID(ctx, toDomainUint64(row.ID), input.LibraryID)
}

// UpdateNodeFields 更新节点元数据字段。
func (r *NodeRepository) UpdateNodeFields(ctx context.Context, nodeID, libraryID uint64, updates map[string]any) (bool, error) {
	q := r.query(ctx)
	info, err := q.Node.WithContext(ctx).
		Where(
			q.Node.ID.Eq(toPGInt64(nodeID)),
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
		).
		Updates(updates)
	if err != nil {
		return false, err
	}
	return info.RowsAffected > 0, nil
}

// RenameNode 在同级目录内重命名节点，并在文件节点时支持扩展名修改。
func (r *NodeRepository) RenameNode(ctx context.Context, nodeID, libraryID uint64, name string, ext *string, updatedAt time.Time) error {
	current, err := r.findNodeModel(ctx, nodeID, libraryID)
	if err != nil {
		return err
	}

	duplicate, err := r.hasDuplicateName(ctx, name, parentIDValue(current.ParentID), libraryID, nodeID)
	if err != nil {
		return err
	}
	if duplicate {
		return ErrConflict
	}

	updates := map[string]any{
		"name":       name,
		"updated_at": updatedAt,
	}
	if current.NodeType == nodeTypeFile && ext != nil {
		trimmedExt := strings.TrimSpace(*ext)
		if trimmedExt == "" {
			updates["ext"] = nil
		} else {
			updates["ext"] = trimmedExt
		}
	}

	updated, err := r.UpdateNodeFields(ctx, nodeID, libraryID, updates)
	if err != nil {
		return err
	}
	if !updated {
		return ErrNotFound
	}
	return nil
}

// MoveNode 处理节点移动与重排，保持同一父级下排序稳定。
func (r *NodeRepository) MoveNode(ctx context.Context, input MoveNodeInput) error {
	node, err := r.findNodeModel(ctx, input.NodeID, input.LibraryID)
	if err != nil {
		return err
	}

	if input.NewParentID == input.NodeID {
		return fmt.Errorf("%w: node cannot be moved under itself", ErrInvalidState)
	}

	if input.NewParentID > 0 {
		newParent, err := r.findNodeModel(ctx, input.NewParentID, input.LibraryID)
		if err != nil {
			return err
		}
		if newParent.NodeType != nodeTypeDirectory {
			return fmt.Errorf("%w: target parent must be directory", ErrInvalidState)
		}
	}

	if input.NewParentID > 0 {
		isDescendant, err := r.isDescendant(ctx, input.NodeID, input.NewParentID, input.LibraryID)
		if err != nil {
			return err
		}
		if isDescendant {
			return fmt.Errorf("%w: cannot move node under descendant", ErrInvalidState)
		}
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = node.Name
	}
	duplicate, err := r.hasDuplicateName(ctx, name, input.NewParentID, input.LibraryID, input.NodeID)
	if err != nil {
		return err
	}
	if duplicate {
		return ErrConflict
	}

	newOrder, err := r.resolveMoveSortOrder(ctx, input)
	if err != nil {
		return err
	}

	updated, err := r.UpdateNodeFields(ctx, input.NodeID, input.LibraryID, map[string]any{
		"parent_id":  normalizePGParentID(input.NewParentID),
		"sort_order": newOrder,
		"updated_at": input.UpdatedAt,
	})
	if err != nil {
		return err
	}
	if !updated {
		return ErrNotFound
	}
	return nil
}

func (r *NodeRepository) resolveMoveSortOrder(ctx context.Context, input MoveNodeInput) (int, error) {
	if input.BeforeNodeID > 0 {
		beforeNode, err := r.findNodeModel(ctx, input.BeforeNodeID, input.LibraryID)
		if err != nil {
			return 0, err
		}
		if parentIDValue(beforeNode.ParentID) != input.NewParentID {
			return 0, fmt.Errorf("%w: before node is not under target parent", ErrInvalidState)
		}

		q := r.query(ctx)
		shift := q.Node.WithContext(ctx).
			Where(
				q.Node.LibraryID.Eq(toPGInt64(input.LibraryID)),
				q.Node.SortOrder.Gte(beforeNode.SortOrder),
			)
		shift = r.applyParentCondition(shift, q, input.NewParentID)
		if _, err := shift.Updates(map[string]any{
			"sort_order": gorm.Expr("sort_order + 1"),
		}); err != nil {
			return 0, err
		}
		return int(beforeNode.SortOrder), nil
	}

	maxSort, err := r.maxSortOrder(ctx, input.LibraryID, input.NewParentID)
	if err != nil {
		return 0, err
	}
	return maxSort + 15, nil
}

func (r *NodeRepository) hasDuplicateName(ctx context.Context, name string, parentID, libraryID, excludeID uint64) (bool, error) {
	q := r.query(ctx)
	query := q.Node.WithContext(ctx).
		Where(
			q.Node.Name.Eq(name),
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
		)
	query = r.applyParentCondition(query, q, parentID)
	if excludeID > 0 {
		query = query.Where(q.Node.ID.Neq(toPGInt64(excludeID)))
	}

	count, err := query.Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *NodeRepository) maxSortOrder(ctx context.Context, libraryID, parentID uint64) (int, error) {
	q := r.query(ctx)
	query := q.Node.WithContext(ctx).
		Where(q.Node.LibraryID.Eq(toPGInt64(libraryID)))
	query = r.applyParentCondition(query, q, parentID)

	row, err := query.
		Order(q.Node.SortOrder.Desc()).
		Limit(1).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return int(row.SortOrder), nil
}

func (r *NodeRepository) isDescendant(ctx context.Context, nodeID, targetID, libraryID uint64) (bool, error) {
	var count int64
	if err := r.scanRaw(ctx, &count, sqlCountSubtreeTargetNode, nodeID, libraryID, libraryID, targetID); err != nil {
		return false, err
	}
	return count > 0, nil
}

func nodeTypeCode(t domainnode.Type) int16 {
	if t == domainnode.TypeFile {
		return nodeTypeFile
	}
	return nodeTypeDirectory
}
