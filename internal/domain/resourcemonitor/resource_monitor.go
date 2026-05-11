package resourcemonitor

import (
	"context"
	"time"
)

// StorageDistributionRow 是仓储层返回的物理存储分布事实。
type StorageDistributionRow struct {
	Provider            string
	Bucket              string
	ObjectCount         int64
	FileRefCount        int64
	PhysicalBytes       int64
	VisibleObjectCount  int64
	VisibleFileRefCount int64
	VisibleBytes        int64
	RecycleObjectCount  int64
	RecycleFileRefCount int64
	RecycleBytes        int64
	OrphanObjectCount   int64
	OrphanBytes         int64
}

// Repository 定义资源监测所需的只读数据端口。
type Repository interface {
	CountStorageDistribution(ctx context.Context, ownerUserID uint64, libraryID uint64) ([]StorageDistributionRow, error)
	Ping(ctx context.Context) error
}

// RedisProbeRepository 定义 Redis 只读探针端口。
type RedisProbeRepository interface {
	Ping(ctx context.Context) error
}

// Snapshot 表示资源监测控制台的当前只读快照。
type Snapshot struct {
	GeneratedAt       time.Time                 `json:"generatedAt"`
	Summary           SnapshotSummary           `json:"summary"`
	Storage           []StorageDistributionItem `json:"storage"`
	DistributionError string                    `json:"distributionError,omitempty"`
	ProbeSummary      ProbeSummary              `json:"probeSummary"`
	Probes            []ProbeTarget             `json:"probes"`
}

// SnapshotSummary 表示快照级汇总。
type SnapshotSummary struct {
	ProviderCount       int   `json:"providerCount"`
	BucketCount         int   `json:"bucketCount"`
	ObjectCount         int64 `json:"objectCount"`
	FileRefCount        int64 `json:"fileRefCount"`
	PhysicalBytes       int64 `json:"physicalBytes"`
	VisibleObjectCount  int64 `json:"visibleObjectCount"`
	VisibleFileRefCount int64 `json:"visibleFileRefCount"`
	VisibleBytes        int64 `json:"visibleBytes"`
	RecycleObjectCount  int64 `json:"recycleObjectCount"`
	RecycleFileRefCount int64 `json:"recycleFileRefCount"`
	RecycleBytes        int64 `json:"recycleBytes"`
	OrphanObjectCount   int64 `json:"orphanObjectCount"`
	OrphanBytes         int64 `json:"orphanBytes"`
	UnmatchedCount      int   `json:"unmatchedCount"`
	LegacyProviderCount int   `json:"legacyProviderCount"`
}

// StorageDistributionItem 表示单个 provider / bucket 的占用。
type StorageDistributionItem struct {
	Provider            string  `json:"provider"`
	SourceProvider      string  `json:"sourceProvider,omitempty"`
	ProviderType        string  `json:"providerType,omitempty"`
	ProviderLabel       string  `json:"providerLabel,omitempty"`
	Endpoint            string  `json:"endpoint,omitempty"`
	Bucket              string  `json:"bucket"`
	IsDefault           bool    `json:"isDefault"`
	IsLegacyProvider    bool    `json:"isLegacyProvider"`
	ObjectCount         int64   `json:"objectCount"`
	FileRefCount        int64   `json:"fileRefCount"`
	PhysicalBytes       int64   `json:"physicalBytes"`
	VisibleObjectCount  int64   `json:"visibleObjectCount"`
	VisibleFileRefCount int64   `json:"visibleFileRefCount"`
	VisibleBytes        int64   `json:"visibleBytes"`
	RecycleObjectCount  int64   `json:"recycleObjectCount"`
	RecycleFileRefCount int64   `json:"recycleFileRefCount"`
	RecycleBytes        int64   `json:"recycleBytes"`
	OrphanObjectCount   int64   `json:"orphanObjectCount"`
	OrphanBytes         int64   `json:"orphanBytes"`
	Percent             float64 `json:"percent"`
	MatchedConfig       bool    `json:"matchedConfig"`
}

// ProbeStatus 表示资源探针结果。
type ProbeStatus string

const (
	ProbeStatusOK      ProbeStatus = "ok"
	ProbeStatusError   ProbeStatus = "error"
	ProbeStatusUnknown ProbeStatus = "unknown"
)

// ProbeSummary 表示探针汇总。
type ProbeSummary struct {
	Total   int `json:"total"`
	OK      int `json:"ok"`
	Error   int `json:"error"`
	Unknown int `json:"unknown"`
}

// ProbeTarget 表示单个物理资源或基础设施资源的探针结果。
type ProbeTarget struct {
	Key          string      `json:"key"`
	Kind         string      `json:"kind"`
	Label        string      `json:"label"`
	Provider     string      `json:"provider,omitempty"`
	ProviderType string      `json:"providerType,omitempty"`
	Endpoint     string      `json:"endpoint,omitempty"`
	Bucket       string      `json:"bucket,omitempty"`
	IsDefault    bool        `json:"isDefault,omitempty"`
	Status       ProbeStatus `json:"status"`
	LatencyMs    int64       `json:"latencyMs"`
	Error        string      `json:"error,omitempty"`
	CheckedAt    time.Time   `json:"checkedAt"`
}
