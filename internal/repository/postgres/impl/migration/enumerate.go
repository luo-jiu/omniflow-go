package repository

import (
	"context"

	domain "omniflow-go/internal/domain/migration"
)

// EnumerateStorageObjectsUnderNode 枚举给定 root_node 子树（含自身）下所有 file 类型节点
// 引用的 distinct storage_object，过滤掉已经位于目标 provider 上的对象。
//
// 返回的列表用于入队迁移子项，调用方再据此生成 target_key、计算 total_bytes。
//
// 实现使用递归 CTE（与 node_sql.go 同套范式）走 nodes → node_files → storage_objects 三表 join，
// 集中收口在仓储层。属于 AGENTS.md 数据访问规范允许的递归 / 复杂聚合豁免。
func (r *MigrationRepository) EnumerateStorageObjectsUnderNode(
	ctx context.Context,
	libraryID, rootNodeID int64,
	excludeProvider string,
) ([]domain.StorageObjectRef, error) {
	const querySQL = `
		WITH RECURSIVE sub AS (
			SELECT id
			FROM nodes
			WHERE id = ? AND library_id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT n.id
			FROM nodes n
			JOIN sub s ON n.parent_id = s.id
			WHERE n.library_id = ? AND n.deleted_at IS NULL
		)
		SELECT DISTINCT
			so.id            AS storage_object_id,
			so.provider      AS provider,
			so.bucket        AS bucket,
			so.object_key    AS object_key,
			so.content_length AS content_length,
			COALESCE(so.content_type, '') AS content_type
		FROM sub
		JOIN node_files nf ON nf.file_id = sub.id AND nf.library_id = ?
		JOIN storage_objects so
		     ON so.id = nf.storage_object_id
		    AND so.library_id = ?
		    AND so.deleted_at IS NULL
		WHERE so.provider <> ?
	`
	type row struct {
		StorageObjectID int64
		Provider        string
		Bucket          string
		ObjectKey       string
		ContentLength   int64
		ContentType     string
	}
	var rows []row
	if err := r.dbWithContext(ctx).
		Raw(querySQL,
			rootNodeID, libraryID,
			libraryID,
			libraryID,
			libraryID,
			excludeProvider,
		).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.StorageObjectRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.StorageObjectRef{
			StorageObjectID: r.StorageObjectID,
			Provider:        r.Provider,
			Bucket:          r.Bucket,
			ObjectKey:       r.ObjectKey,
			ContentLength:   r.ContentLength,
			ContentType:     r.ContentType,
		})
	}
	return out, nil
}

// CountStorageDistribution 给迁移 dialog 用，
// 返回 root_node 子树下按 provider 聚合的 (file_count, total_bytes)。
func (r *MigrationRepository) CountStorageDistribution(
	ctx context.Context,
	libraryID, rootNodeID int64,
) ([]ProviderDistribution, error) {
	const querySQL = `
		WITH RECURSIVE sub AS (
			SELECT id
			FROM nodes
			WHERE id = ? AND library_id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT n.id
			FROM nodes n
			JOIN sub s ON n.parent_id = s.id
			WHERE n.library_id = ? AND n.deleted_at IS NULL
		)
		SELECT
			so.provider AS provider,
			COUNT(DISTINCT so.id) AS file_count,
			COALESCE(SUM(so.content_length), 0) AS total_bytes
		FROM sub
		JOIN node_files nf ON nf.file_id = sub.id AND nf.library_id = ?
		JOIN storage_objects so
		     ON so.id = nf.storage_object_id
		    AND so.library_id = ?
		    AND so.deleted_at IS NULL
		GROUP BY so.provider
	`
	var rows []ProviderDistribution
	if err := r.dbWithContext(ctx).
		Raw(querySQL, rootNodeID, libraryID, libraryID, libraryID, libraryID).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ProviderDistribution 表示某 provider 下的对象分布。
type ProviderDistribution struct {
	Provider   string `json:"provider"`
	FileCount  int64  `json:"file_count"`
	TotalBytes int64  `json:"total_bytes"`
}
