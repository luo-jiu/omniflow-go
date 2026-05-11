package resourcemonitor

import (
	"context"
	"time"
)

// StorageDistributionRow 是仓储层返回的物理存储分布事实。
type StorageDistributionRow struct {
	Provider      string
	Bucket        string
	ObjectCount   int64
	FileRefCount  int64
	PhysicalBytes int64
}

// Repository 定义资源监测所需的只读数据端口。
type Repository interface {
	CountStorageDistribution(ctx context.Context, ownerUserID uint64) ([]StorageDistributionRow, error)
}

// Snapshot 表示资源监测控制台的当前只读快照。
type Snapshot struct {
	GeneratedAt time.Time                 `json:"generatedAt"`
	Summary     SnapshotSummary           `json:"summary"`
	Storage     []StorageDistributionItem `json:"storage"`
}

// SnapshotSummary 表示快照级汇总。
type SnapshotSummary struct {
	ProviderCount  int   `json:"providerCount"`
	BucketCount    int   `json:"bucketCount"`
	ObjectCount    int64 `json:"objectCount"`
	FileRefCount   int64 `json:"fileRefCount"`
	PhysicalBytes  int64 `json:"physicalBytes"`
	UnmatchedCount int   `json:"unmatchedCount"`
}

// StorageDistributionItem 表示单个 provider / bucket 的占用。
type StorageDistributionItem struct {
	Provider      string  `json:"provider"`
	ProviderType  string  `json:"providerType,omitempty"`
	ProviderLabel string  `json:"providerLabel,omitempty"`
	Endpoint      string  `json:"endpoint,omitempty"`
	Bucket        string  `json:"bucket"`
	IsDefault     bool    `json:"isDefault"`
	ObjectCount   int64   `json:"objectCount"`
	FileRefCount  int64   `json:"fileRefCount"`
	PhysicalBytes int64   `json:"physicalBytes"`
	Percent       float64 `json:"percent"`
	MatchedConfig bool    `json:"matchedConfig"`
}
