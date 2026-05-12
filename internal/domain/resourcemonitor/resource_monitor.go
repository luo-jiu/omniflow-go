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

// BreakdownLibraryRow 是仓储层返回的资料库维度细分统计事实。
type BreakdownLibraryRow struct {
	LibraryID             int64
	LibraryName           string
	ObjectCount           int64
	FileRefCount          int64
	PhysicalBytes         int64
	ReferencedBytes       int64
	VisibleObjectCount    int64
	VisibleFileRefCount   int64
	VisibleBytes          int64
	RecycleObjectCount    int64
	RecycleFileRefCount   int64
	RecycleBytes          int64
	OrphanObjectCount     int64
	OrphanBytes           int64
	ArchiveDirectoryCount int64
	MultiRefObjectCount   int64
	MultiRefPhysicalBytes int64
	TopProvider           string
	TopBucket             string
}

// BreakdownCategoryRow 是仓储层返回的归档分类维度细分统计事实。
type BreakdownCategoryRow struct {
	Key                   string
	BuiltInType           string
	ObjectCount           int64
	FileRefCount          int64
	PhysicalBytes         int64
	ReferencedBytes       int64
	VisibleObjectCount    int64
	VisibleFileRefCount   int64
	VisibleBytes          int64
	RecycleObjectCount    int64
	RecycleFileRefCount   int64
	RecycleBytes          int64
	OrphanObjectCount     int64
	OrphanBytes           int64
	ArchiveDirectoryCount int64
}

// Repository 定义资源监测所需的只读数据端口。
type Repository interface {
	LibraryBelongsToOwner(ctx context.Context, ownerUserID uint64, libraryID uint64) (bool, error)
	CountStorageDistribution(ctx context.Context, ownerUserID uint64, libraryID uint64) ([]StorageDistributionRow, error)
	CountBreakdownLibraries(ctx context.Context, ownerUserID uint64, libraryID uint64) ([]BreakdownLibraryRow, error)
	CountBreakdownCategories(ctx context.Context, ownerUserID uint64, libraryID uint64) ([]BreakdownCategoryRow, error)
	SaveSample(ctx context.Context, sample Sample) (Sample, error)
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

// Breakdown 表示资源监测控制台的细分仪表盘快照。
type Breakdown struct {
	GeneratedAt    time.Time               `json:"generatedAt"`
	Summary        BreakdownSummary        `json:"summary"`
	Libraries      []BreakdownLibraryItem  `json:"libraries"`
	Categories     []BreakdownCategoryItem `json:"categories"`
	Statuses       []BreakdownStatusItem   `json:"statuses"`
	Anomalies      []BreakdownAnomalyItem  `json:"anomalies"`
	BreakdownError string                  `json:"breakdownError,omitempty"`
}

// BreakdownSummary 表示细分仪表盘的总览指标。
type BreakdownSummary struct {
	LibraryCount          int   `json:"libraryCount"`
	ArchiveDirectoryCount int64 `json:"archiveDirectoryCount"`
	PhysicalBytes         int64 `json:"physicalBytes"`
	ReferencedBytes       int64 `json:"referencedBytes"`
	ObjectCount           int64 `json:"objectCount"`
	FileRefCount          int64 `json:"fileRefCount"`
	VisibleObjectCount    int64 `json:"visibleObjectCount"`
	VisibleFileRefCount   int64 `json:"visibleFileRefCount"`
	VisibleBytes          int64 `json:"visibleBytes"`
	RecycleObjectCount    int64 `json:"recycleObjectCount"`
	RecycleFileRefCount   int64 `json:"recycleFileRefCount"`
	RecycleBytes          int64 `json:"recycleBytes"`
	OrphanObjectCount     int64 `json:"orphanObjectCount"`
	OrphanBytes           int64 `json:"orphanBytes"`
	MultiRefObjectCount   int64 `json:"multiRefObjectCount"`
	MultiRefPhysicalBytes int64 `json:"multiRefPhysicalBytes"`
}

// BreakdownLibraryItem 表示单个资料库的资源细分。
type BreakdownLibraryItem struct {
	LibraryID             int64   `json:"libraryId"`
	LibraryName           string  `json:"libraryName"`
	PhysicalBytes         int64   `json:"physicalBytes"`
	ReferencedBytes       int64   `json:"referencedBytes"`
	ObjectCount           int64   `json:"objectCount"`
	FileRefCount          int64   `json:"fileRefCount"`
	ArchiveDirectoryCount int64   `json:"archiveDirectoryCount"`
	VisibleBytes          int64   `json:"visibleBytes"`
	RecycleBytes          int64   `json:"recycleBytes"`
	OrphanBytes           int64   `json:"orphanBytes"`
	TopProvider           string  `json:"topProvider,omitempty"`
	TopBucket             string  `json:"topBucket,omitempty"`
	Percent               float64 `json:"percent"`
}

// BreakdownCategoryItem 表示单个归档分类的资源细分。
type BreakdownCategoryItem struct {
	Key                   string  `json:"key"`
	Label                 string  `json:"label"`
	BuiltInType           string  `json:"builtInType,omitempty"`
	PhysicalBytes         int64   `json:"physicalBytes"`
	ReferencedBytes       int64   `json:"referencedBytes"`
	ObjectCount           int64   `json:"objectCount"`
	FileRefCount          int64   `json:"fileRefCount"`
	ArchiveDirectoryCount int64   `json:"archiveDirectoryCount"`
	VisibleBytes          int64   `json:"visibleBytes"`
	RecycleBytes          int64   `json:"recycleBytes"`
	OrphanBytes           int64   `json:"orphanBytes"`
	Percent               float64 `json:"percent"`
}

// BreakdownStatusItem 表示 visible / recycle / orphan 状态维度的资源细分。
type BreakdownStatusItem struct {
	Key           string  `json:"key"`
	Label         string  `json:"label"`
	PhysicalBytes int64   `json:"physicalBytes"`
	ObjectCount   int64   `json:"objectCount"`
	FileRefCount  int64   `json:"fileRefCount"`
	Percent       float64 `json:"percent"`
}

// BreakdownAnomalyItem 表示资源监测仪表盘的只读诊断项。
type BreakdownAnomalyItem struct {
	Key           string `json:"key"`
	Severity      string `json:"severity"`
	Title         string `json:"title"`
	Message       string `json:"message"`
	LibraryID     int64  `json:"libraryId,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Bucket        string `json:"bucket,omitempty"`
	PhysicalBytes int64  `json:"physicalBytes,omitempty"`
	ObjectCount   int64  `json:"objectCount,omitempty"`
}

// Sample 表示一条资源监测历史采样。
type Sample struct {
	ID                  int64     `json:"id"`
	DryRun              bool      `json:"dryRun"`
	ActorID             string    `json:"actorId"`
	Scope               string    `json:"scope"`
	LibraryID           int64     `json:"libraryId"`
	GeneratedAt         time.Time `json:"generatedAt"`
	ProviderCount       int       `json:"providerCount"`
	BucketCount         int       `json:"bucketCount"`
	ObjectCount         int64     `json:"objectCount"`
	FileRefCount        int64     `json:"fileRefCount"`
	PhysicalBytes       int64     `json:"physicalBytes"`
	VisibleObjectCount  int64     `json:"visibleObjectCount"`
	VisibleFileRefCount int64     `json:"visibleFileRefCount"`
	VisibleBytes        int64     `json:"visibleBytes"`
	RecycleObjectCount  int64     `json:"recycleObjectCount"`
	RecycleFileRefCount int64     `json:"recycleFileRefCount"`
	RecycleBytes        int64     `json:"recycleBytes"`
	OrphanObjectCount   int64     `json:"orphanObjectCount"`
	OrphanBytes         int64     `json:"orphanBytes"`
	UnmatchedCount      int       `json:"unmatchedCount"`
	LegacyProviderCount int       `json:"legacyProviderCount"`
	ProbeTotal          int       `json:"probeTotal"`
	ProbeOK             int       `json:"probeOk"`
	ProbeError          int       `json:"probeError"`
	ProbeUnknown        int       `json:"probeUnknown"`
	DistributionError   string    `json:"distributionError,omitempty"`
	SnapshotJSON        string    `json:"-"`
	CreatedAt           time.Time `json:"createdAt"`
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
