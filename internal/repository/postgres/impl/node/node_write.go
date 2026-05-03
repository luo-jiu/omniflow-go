package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
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

const (
	moveSortStep        = 1024
	moveMinSortGap      = 2
	moveMaxInt32        = int(^uint32(0) >> 1)
	moveMaxSafeSortBase = moveMaxInt32 - moveSortStep*4
)

// CreateNode 创建节点并在文件类型时补齐对象存储关联。
func (r *NodeRepository) CreateNode(ctx context.Context, input CreateNodeInput) (domainnode.Node, error) {
	name := input.Name
	if strings.TrimSpace(name) == "" {
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

	maxSort, err := r.maxSortOrder(ctx, input.LibraryID, input.ParentID)
	if err != nil {
		return domainnode.Node{}, err
	}

	var ext *string
	createStorageBinding := false
	if input.Type == domainnode.TypeFile {
		trimmedExt := strings.TrimSpace(input.Ext)
		if trimmedExt != "" {
			ext = &trimmedExt
		}
		if input.FileSize < 0 {
			return domainnode.Node{}, fmt.Errorf("%w: file size must be >= 0", ErrInvalidState)
		}
		createStorageBinding = strings.TrimSpace(input.StorageKey) != ""
	}

	duplicate, err := r.hasDuplicateVisibleName(
		ctx,
		name,
		derefString(ext),
		input.Type,
		input.ParentID,
		input.LibraryID,
		0,
	)
	if err != nil {
		return domainnode.Node{}, err
	}
	if duplicate {
		return domainnode.Node{}, ErrConflict
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
		return domainnode.Node{}, mapDBError(err)
	}

	if input.Type == domainnode.TypeFile && createStorageBinding {
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
			return domainnode.Node{}, mapDBError(err)
		}

		fileRow := &pgmodel.NodeFile{
			FileID:          row.ID,
			LibraryID:       toPGInt64(input.LibraryID),
			StorageObjectID: storageRow.ID,
			MimeType:        nullableString(strings.TrimSpace(input.MIMEType)),
			FileSize:        input.FileSize,
		}
		if err := q.NodeFile.WithContext(ctx).Create(fileRow); err != nil {
			return domainnode.Node{}, mapDBError(err)
		}
	}

	return r.FindViewByID(ctx, toDomainUint64(row.ID), input.LibraryID)
}

// LockSiblingNameScope 对同一资料库、同一父节点下的命名空间加事务级锁。
func (r *NodeRepository) LockSiblingNameScope(ctx context.Context, libraryID, parentID uint64) error {
	scope := fmt.Sprintf("nodes:name:%d:%d", libraryID, parentID)
	return r.dbWithContext(ctx).Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", scope).Error
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

// BatchSetDirectChildDirectoriesBuiltInType 批量设置直属子目录的内置类型。
func (r *NodeRepository) BatchSetDirectChildDirectoriesBuiltInType(
	ctx context.Context,
	parentNodeID uint64,
	libraryID uint64,
	builtInType string,
	updatedAt time.Time,
) (int64, error) {
	q := r.query(ctx)
	info, err := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.ParentID.Eq(toPGInt64(parentNodeID)),
			q.Node.NodeType.Eq(nodeTypeDirectory),
			q.Node.BuiltInType.Neq(builtInType),
		).
		Updates(map[string]any{
			"built_in_type": builtInType,
			"updated_at":    updatedAt,
		})
	if err != nil {
		return 0, err
	}
	return info.RowsAffected, nil
}

// RenameNode 在同级目录内重命名节点，并在文件节点时支持扩展名修改。
func (r *NodeRepository) RenameNode(ctx context.Context, nodeID, libraryID uint64, name string, ext *string, updatedAt time.Time) error {
	current, err := r.findNodeModel(ctx, nodeID, libraryID)
	if err != nil {
		return err
	}

	updates := map[string]any{
		"name":       name,
		"updated_at": updatedAt,
	}
	nextExt := derefString(current.Ext)
	if current.NodeType == nodeTypeFile && ext != nil {
		trimmedExt := strings.TrimSpace(*ext)
		nextExt = trimmedExt
		if trimmedExt == "" {
			updates["ext"] = nil
		} else {
			updates["ext"] = trimmedExt
		}
	}

	duplicate, err := r.hasDuplicateVisibleName(
		ctx,
		name,
		nextExt,
		domainTypeCode(current.NodeType),
		parentIDValue(current.ParentID),
		libraryID,
		nodeID,
	)
	if err != nil {
		return err
	}
	if duplicate {
		return ErrConflict
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
	oldParentID := parentIDValue(node.ParentID)

	if input.NewParentID == input.NodeID {
		return fmt.Errorf("%w: node cannot be moved under itself", ErrInvalidState)
	}

	if err := r.lockMoveScope(ctx, input.LibraryID, oldParentID, input.NewParentID); err != nil {
		return err
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

	name := input.Name
	if strings.TrimSpace(name) == "" {
		name = node.Name
	}
	duplicate, err := r.hasDuplicateVisibleName(
		ctx,
		name,
		derefString(node.Ext),
		domainTypeCode(node.NodeType),
		input.NewParentID,
		input.LibraryID,
		input.NodeID,
	)
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

		left, right, err := r.resolveMoveBeforeRange(ctx, input.NewParentID, input.LibraryID, beforeNode)
		if err != nil {
			return 0, err
		}
		if right-left <= 1 {
			if err := r.incrementSortOrderAfter(ctx, input.NewParentID, input.LibraryID, right); err != nil {
				return 0, err
			}
			return right, nil
		}
		return left + ((right - left) / 2), nil
	}

	maxSort, err := r.maxSortOrder(ctx, input.LibraryID, input.NewParentID)
	if err != nil {
		return 0, err
	}
	if maxSort >= moveMaxSafeSortBase {
		if err := r.reindexParent(ctx, input.NewParentID, input.LibraryID, input.UpdatedAt); err != nil {
			return 0, err
		}
		maxSort, err = r.maxSortOrder(ctx, input.LibraryID, input.NewParentID)
		if err != nil {
			return 0, err
		}
	}
	if maxSort <= 0 {
		return moveSortStep, nil
	}
	return maxSort + moveSortStep, nil
}

func (r *NodeRepository) hasDuplicateVisibleName(
	ctx context.Context,
	name string,
	ext string,
	nodeType domainnode.Type,
	parentID, libraryID, excludeID uint64,
) (bool, error) {
	visibleName := buildVisibleName(name, ext, nodeTypeCode(nodeType))
	query := r.dbWithContext(ctx).
		Model(&pgmodel.Node{}).
		Where("library_id = ?", toPGInt64(libraryID)).
		Where(
			"CASE WHEN node_type = ? AND COALESCE(ext, '') <> '' THEN name || '.' || ext ELSE name END = ?",
			nodeTypeFile,
			visibleName,
		)
	if parentID == 0 {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", toPGInt64(parentID))
	}
	if excludeID > 0 {
		query = query.Where("id <> ?", toPGInt64(excludeID))
	}

	var count int64
	err := query.Count(&count).Error
	return count > 0, err
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

// ReplaceFileStorageInput 文件存储替换参数。
type ReplaceFileStorageInput struct {
	NewObjectKey   string
	NewFileSize    int64
	NewContentType string
	NewProvider    string
	NewBucket      string
}

// FindFileByNameInParent 在同级目录中按文件名主体与扩展名查找文件节点（不含已删除）。
func (r *NodeRepository) FindFileByNameInParent(
	ctx context.Context,
	parentID, libraryID uint64,
	name string,
	ext string,
) (*pgmodel.Node, error) {
	query := r.dbWithContext(ctx).
		Model(&pgmodel.Node{}).
		Where(
			"name = ? AND library_id = ? AND node_type = ?",
			name,
			toPGInt64(libraryID),
			nodeTypeFile,
		)
	normalizedExt := strings.TrimPrefix(strings.TrimSpace(ext), ".")
	if normalizedExt == "" {
		query = query.Where("(ext IS NULL OR ext = '')")
	} else {
		query = query.Where("ext = ?", normalizedExt)
	}
	if parentID == 0 {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", toPGInt64(parentID))
	}

	var row pgmodel.Node
	err := query.First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// ReplaceFileStorage 替换文件节点的存储绑定，返回旧的 object_key 用于清理。
func (r *NodeRepository) ReplaceFileStorage(
	ctx context.Context,
	nodeID, libraryID uint64,
	input ReplaceFileStorageInput,
) (string, error) {
	q := r.query(ctx)
	now := time.Now().UTC()

	// 1. 查找 node_files 记录；历史上右键新建文件可能只有节点元数据，没有存储绑定。
	nodeFile, err := q.NodeFile.WithContext(ctx).
		Where(
			q.NodeFile.FileID.Eq(toPGInt64(nodeID)),
			q.NodeFile.LibraryID.Eq(toPGInt64(libraryID)),
		).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.attachFileStorage(ctx, nodeID, libraryID, input, now)
		}
		return "", fmt.Errorf("find node_files: %w", mapDBError(err))
	}

	// 2. 查找 storage_objects 记录，获取旧 object_key
	storageObj, err := q.StorageObject.WithContext(ctx).
		Where(q.StorageObject.ID.Eq(nodeFile.StorageObjectID)).
		First()
	if err != nil {
		return "", fmt.Errorf("find storage_object: %w", mapDBError(err))
	}
	oldObjectKey := storageObj.ObjectKey

	// 3. 更新 storage_objects
	_, err = q.StorageObject.WithContext(ctx).
		Where(q.StorageObject.ID.Eq(storageObj.ID)).
		Updates(map[string]any{
			"object_key":     strings.TrimSpace(input.NewObjectKey),
			"content_length": input.NewFileSize,
			"content_type":   nullableString(strings.TrimSpace(input.NewContentType)),
			"provider":       strings.TrimSpace(input.NewProvider),
			"bucket":         strings.TrimSpace(input.NewBucket),
			"updated_at":     now,
		})
	if err != nil {
		return "", fmt.Errorf("update storage_object: %w", mapDBError(err))
	}

	// 4. 更新 node_files
	_, err = q.NodeFile.WithContext(ctx).
		Where(
			q.NodeFile.FileID.Eq(toPGInt64(nodeID)),
			q.NodeFile.LibraryID.Eq(toPGInt64(libraryID)),
		).
		Updates(map[string]any{
			"file_size":  input.NewFileSize,
			"mime_type":  nullableString(strings.TrimSpace(input.NewContentType)),
			"updated_at": now,
		})
	if err != nil {
		return "", fmt.Errorf("update node_files: %w", mapDBError(err))
	}

	// 5. 更新节点的 updated_at
	_, err = q.Node.WithContext(ctx).
		Where(
			q.Node.ID.Eq(toPGInt64(nodeID)),
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
		).
		Updates(map[string]any{
			"updated_at": now,
		})
	if err != nil {
		return "", fmt.Errorf("update node: %w", mapDBError(err))
	}

	return oldObjectKey, nil
}

func (r *NodeRepository) attachFileStorage(
	ctx context.Context,
	nodeID, libraryID uint64,
	input ReplaceFileStorageInput,
	now time.Time,
) (string, error) {
	q := r.query(ctx)
	node, err := q.Node.WithContext(ctx).
		Where(
			q.Node.ID.Eq(toPGInt64(nodeID)),
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
		).
		First()
	if err != nil {
		return "", fmt.Errorf("find node: %w", mapDBError(err))
	}
	if node.NodeType != nodeTypeFile {
		return "", fmt.Errorf("%w: node is not a file", ErrInvalidState)
	}

	storageRow := &pgmodel.StorageObject{
		LibraryID:     toPGInt64(libraryID),
		Provider:      strings.TrimSpace(input.NewProvider),
		Bucket:        strings.TrimSpace(input.NewBucket),
		ObjectKey:     strings.TrimSpace(input.NewObjectKey),
		ContentLength: input.NewFileSize,
		ContentType:   nullableString(strings.TrimSpace(input.NewContentType)),
		Extra:         "{}",
	}
	if err := q.StorageObject.WithContext(ctx).Create(storageRow); err != nil {
		return "", fmt.Errorf("create storage_object: %w", mapDBError(err))
	}

	fileRow := &pgmodel.NodeFile{
		FileID:          toPGInt64(nodeID),
		LibraryID:       toPGInt64(libraryID),
		StorageObjectID: storageRow.ID,
		MimeType:        nullableString(strings.TrimSpace(input.NewContentType)),
		FileSize:        input.NewFileSize,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := q.NodeFile.WithContext(ctx).Create(fileRow); err != nil {
		return "", fmt.Errorf("create node_files: %w", mapDBError(err))
	}

	_, err = q.Node.WithContext(ctx).
		Where(
			q.Node.ID.Eq(toPGInt64(nodeID)),
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
		).
		Updates(map[string]any{
			"updated_at": now,
		})
	if err != nil {
		return "", fmt.Errorf("update node: %w", mapDBError(err))
	}

	return "", nil
}

func nodeTypeCode(t domainnode.Type) int16 {
	if t == domainnode.TypeFile {
		return nodeTypeFile
	}
	return nodeTypeDirectory
}

func domainTypeCode(t int16) domainnode.Type {
	if t == nodeTypeFile {
		return domainnode.TypeFile
	}
	return domainnode.TypeDirectory
}

func buildVisibleName(name string, ext string, nodeType int16) string {
	trimmedName := strings.TrimSpace(name)
	normalizedExt := strings.TrimPrefix(strings.TrimSpace(ext), ".")
	if nodeType == nodeTypeFile && normalizedExt != "" {
		return trimmedName + "." + normalizedExt
	}
	return trimmedName
}

func (r *NodeRepository) resolveMoveBeforeRange(
	ctx context.Context,
	parentID, libraryID uint64,
	beforeNode *pgmodel.Node,
) (int, int, error) {
	leftSort, err := r.prevSortOrderBeforeNode(ctx, parentID, libraryID, beforeNode.SortOrder, toDomainUint64(beforeNode.ID))
	if err != nil {
		return 0, 0, err
	}
	left := 0
	if leftSort != nil {
		left = *leftSort
	}
	right := int(beforeNode.SortOrder)

	if right-left <= moveMinSortGap {
		if err := r.reindexParent(ctx, parentID, libraryID, time.Now().UTC()); err != nil {
			return 0, 0, err
		}
		refreshedBefore, err := r.findNodeModel(ctx, toDomainUint64(beforeNode.ID), libraryID)
		if err != nil {
			return 0, 0, err
		}
		leftSort, err = r.prevSortOrderBeforeNode(
			ctx,
			parentID,
			libraryID,
			refreshedBefore.SortOrder,
			toDomainUint64(refreshedBefore.ID),
		)
		if err != nil {
			return 0, 0, err
		}
		left = 0
		if leftSort != nil {
			left = *leftSort
		}
		right = int(refreshedBefore.SortOrder)
	}
	return left, right, nil
}

func (r *NodeRepository) prevSortOrderBeforeNode(
	ctx context.Context,
	parentID, libraryID uint64,
	beforeSort int32,
	beforeID uint64,
) (*int, error) {
	q := r.query(ctx)
	query := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.SortOrder.Lt(beforeSort),
		)
	query = r.applyParentCondition(query, q, parentID)

	row, err := query.
		Order(q.Node.SortOrder.Desc(), q.Node.ID.Desc()).
		Limit(1).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if toDomainUint64(row.ID) == beforeID {
		return nil, nil
	}
	value := int(row.SortOrder)
	return &value, nil
}

func (r *NodeRepository) reindexParent(ctx context.Context, parentID, libraryID uint64, updatedAt time.Time) error {
	q := r.query(ctx)
	query := q.Node.WithContext(ctx).
		Where(q.Node.LibraryID.Eq(toPGInt64(libraryID)))
	query = r.applyParentCondition(query, q, parentID)

	siblings, err := query.
		Order(q.Node.SortOrder.Asc(), q.Node.ID.Asc()).
		Find()
	if err != nil {
		return err
	}
	if len(siblings) == 0 {
		return nil
	}

	order := moveSortStep
	for _, sibling := range siblings {
		if order > moveMaxInt32-moveSortStep {
			return fmt.Errorf("%w: sort order range exhausted under parent %d", ErrInvalidState, parentID)
		}
		info, err := q.Node.WithContext(ctx).
			Where(
				q.Node.ID.Eq(sibling.ID),
				q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			).
			Updates(map[string]any{
				"sort_order": order,
				"updated_at": updatedAt,
			})
		if err != nil {
			return err
		}
		if info.RowsAffected == 0 {
			return ErrNotFound
		}
		order += moveSortStep
	}
	return nil
}

func (r *NodeRepository) incrementSortOrderAfter(ctx context.Context, parentID, libraryID uint64, rightSort int) error {
	q := r.query(ctx)
	query := q.Node.WithContext(ctx).
		Where(
			q.Node.LibraryID.Eq(toPGInt64(libraryID)),
			q.Node.SortOrder.Gte(int32(rightSort)),
		)
	query = r.applyParentCondition(query, q, parentID)
	_, err := query.Updates(map[string]any{
		"sort_order": gorm.Expr("sort_order + 1"),
	})
	return err
}

func (r *NodeRepository) lockMoveScope(ctx context.Context, libraryID, oldParentID, newParentID uint64) error {
	scopes := []string{
		fmt.Sprintf("nodes:move:parent:%d:%d", libraryID, oldParentID),
		fmt.Sprintf("nodes:move:children:%d:%d", libraryID, oldParentID),
		fmt.Sprintf("nodes:move:parent:%d:%d", libraryID, newParentID),
		fmt.Sprintf("nodes:move:children:%d:%d", libraryID, newParentID),
	}

	unique := make([]string, 0, len(scopes))
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		unique = append(unique, scope)
	}
	sort.Strings(unique)

	db := r.dbWithContext(ctx)
	for _, scope := range unique {
		if err := db.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", scope).Error; err != nil {
			return err
		}
	}
	return nil
}
