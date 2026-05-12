package usecase

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/config"
	domain "omniflow-go/internal/domain/resourcemonitor"
	"omniflow-go/internal/storage"
)

type fakeResourceMonitorRepository struct {
	rows         []domain.StorageDistributionRow
	libraryRows  []domain.BreakdownLibraryRow
	categoryRows []domain.BreakdownCategoryRow
	matrixRows   []domain.DashboardMatrixRow
	err          error
	categoryErr  error
	matrixErr    error
	ping         error
	got          uint64
	gotLibrary   uint64
	ownedBy      map[uint64]bool
	saved        domain.Sample
}

func (r *fakeResourceMonitorRepository) LibraryBelongsToOwner(
	_ context.Context,
	ownerUserID uint64,
	libraryID uint64,
) (bool, error) {
	r.got = ownerUserID
	if r.ownedBy == nil {
		return true, nil
	}
	return r.ownedBy[libraryID], nil
}

func (r *fakeResourceMonitorRepository) CountStorageDistribution(
	_ context.Context,
	ownerUserID uint64,
	libraryID uint64,
) ([]domain.StorageDistributionRow, error) {
	r.got = ownerUserID
	r.gotLibrary = libraryID
	if r.err != nil {
		return nil, r.err
	}
	return r.rows, nil
}

func (r *fakeResourceMonitorRepository) CountBreakdownLibraries(
	_ context.Context,
	ownerUserID uint64,
	libraryID uint64,
) ([]domain.BreakdownLibraryRow, error) {
	r.got = ownerUserID
	r.gotLibrary = libraryID
	if r.err != nil {
		return nil, r.err
	}
	return r.libraryRows, nil
}

func (r *fakeResourceMonitorRepository) CountBreakdownCategories(
	_ context.Context,
	ownerUserID uint64,
	libraryID uint64,
) ([]domain.BreakdownCategoryRow, error) {
	r.got = ownerUserID
	r.gotLibrary = libraryID
	if r.categoryErr != nil {
		return nil, r.categoryErr
	}
	return r.categoryRows, nil
}

func (r *fakeResourceMonitorRepository) CountDashboardMatrix(
	_ context.Context,
	ownerUserID uint64,
	libraryID uint64,
) ([]domain.DashboardMatrixRow, error) {
	r.got = ownerUserID
	r.gotLibrary = libraryID
	if r.matrixErr != nil {
		return nil, r.matrixErr
	}
	return r.matrixRows, nil
}

func (r *fakeResourceMonitorRepository) Ping(_ context.Context) error {
	return r.ping
}

func (r *fakeResourceMonitorRepository) SaveSample(
	_ context.Context,
	sample domain.Sample,
) (domain.Sample, error) {
	r.saved = sample
	sample.ID = 1
	sample.CreatedAt = time.Now().UTC()
	return sample, nil
}

type fakeRedisProbeRepository struct {
	err error
}

func (r *fakeRedisProbeRepository) Ping(_ context.Context) error {
	return r.err
}

type fakeObjectStorage struct {
	bucket string
	err    error
}

func (s fakeObjectStorage) Upload(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (s fakeObjectStorage) GetPresignedURL(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

func (s fakeObjectStorage) Delete(context.Context, string) error {
	return nil
}

func (s fakeObjectStorage) Bucket() string {
	return s.bucket
}

func (s fakeObjectStorage) Probe(context.Context) error {
	return s.err
}

func (s fakeObjectStorage) PresignedPutObject(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

func (s fakeObjectStorage) StatObject(context.Context, string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{}, nil
}

func (s fakeObjectStorage) GetObject(context.Context, string) (io.ReadCloser, storage.ObjectInfo, error) {
	return nil, storage.ObjectInfo{}, nil
}

func (s fakeObjectStorage) InitiateMultipartUpload(context.Context, string, string) (string, error) {
	return "", nil
}

func (s fakeObjectStorage) UploadPart(context.Context, string, string, int, io.Reader, int64) (string, error) {
	return "", nil
}

func (s fakeObjectStorage) PresignedUploadPart(context.Context, string, string, int, time.Duration) (string, error) {
	return "", nil
}

func (s fakeObjectStorage) CompleteMultipartUpload(
	context.Context,
	string,
	string,
	[]storage.MultipartUploadPart,
) error {
	return nil
}

func (s fakeObjectStorage) AbortMultipartUpload(context.Context, string, string) error {
	return nil
}

func (s fakeObjectStorage) ListParts(context.Context, string, string) ([]storage.MultipartUploadPart, error) {
	return nil, nil
}

func TestResourceMonitorSnapshot(t *testing.T) {
	repo := &fakeResourceMonitorRepository{
		rows: []domain.StorageDistributionRow{
			{
				Provider:            "local-minio",
				Bucket:              "default",
				ObjectCount:         2,
				FileRefCount:        3,
				PhysicalBytes:       1024,
				VisibleObjectCount:  2,
				VisibleFileRefCount: 3,
				VisibleBytes:        1024,
			},
			{
				Provider:            "unknown-provider",
				Bucket:              "legacy",
				ObjectCount:         1,
				FileRefCount:        1,
				PhysicalBytes:       512,
				RecycleObjectCount:  1,
				RecycleFileRefCount: 1,
				RecycleBytes:        512,
			},
			{
				Provider:          "MINIO",
				Bucket:            "default",
				ObjectCount:       1,
				FileRefCount:      2,
				PhysicalBytes:     256,
				OrphanObjectCount: 1,
				OrphanBytes:       256,
			},
		},
	}
	registry := storage.NewStorageRegistry()
	_, err := registry.Reload(&config.StorageConfig{
		Providers: map[string]config.ProviderConfig{
			"local-minio": {
				Type:     "MINIO",
				Endpoint: "127.0.0.1:9000",
				Bucket:   "default",
				Label:    "Local MinIO",
			},
		},
		DefaultProvider: "local-minio",
	}, func(alias string, cfg config.ProviderConfig) (storage.ObjectStorage, func(), error) {
		return fakeObjectStorage{bucket: cfg.Bucket}, func() {}, nil
	})
	if err != nil {
		t.Fatalf("reload registry: %v", err)
	}

	uc := NewResourceMonitorUseCase(repo, &fakeRedisProbeRepository{}, registry)
	got, err := uc.Snapshot(context.Background(), actor.Actor{ID: "42", Kind: actor.KindUser})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	if repo.got != 42 {
		t.Fatalf("ownerUserID = %d, want 42", repo.got)
	}
	if repo.gotLibrary != 0 {
		t.Fatalf("libraryID = %d, want 0", repo.gotLibrary)
	}
	if got.Summary.ProviderCount != 2 || got.Summary.BucketCount != 2 {
		t.Fatalf("summary provider/bucket = %d/%d, want 2/2", got.Summary.ProviderCount, got.Summary.BucketCount)
	}
	if got.Summary.ObjectCount != 4 || got.Summary.FileRefCount != 6 || got.Summary.PhysicalBytes != 1792 {
		t.Fatalf("summary counts = %+v", got.Summary)
	}
	if got.Summary.VisibleObjectCount != 2 || got.Summary.VisibleFileRefCount != 3 || got.Summary.VisibleBytes != 1024 {
		t.Fatalf("visible summary = %+v", got.Summary)
	}
	if got.Summary.RecycleObjectCount != 1 || got.Summary.RecycleFileRefCount != 1 || got.Summary.RecycleBytes != 512 {
		t.Fatalf("recycle summary = %+v", got.Summary)
	}
	if got.Summary.OrphanObjectCount != 1 || got.Summary.OrphanBytes != 256 {
		t.Fatalf("orphan summary = %+v", got.Summary)
	}
	if got.Summary.UnmatchedCount != 1 {
		t.Fatalf("unmatched = %d, want 1", got.Summary.UnmatchedCount)
	}
	if got.Summary.LegacyProviderCount != 1 {
		t.Fatalf("legacy provider count = %d, want 1", got.Summary.LegacyProviderCount)
	}
	if len(got.Storage) != 2 {
		t.Fatalf("storage len = %d, want 2", len(got.Storage))
	}
	if got.Storage[0].Provider != "local-minio" || !got.Storage[0].IsDefault || got.Storage[0].Percent != 71.4 {
		t.Fatalf("first storage = %+v", got.Storage[0])
	}
	if got.Storage[0].ObjectCount != 3 || got.Storage[0].FileRefCount != 5 || got.Storage[0].PhysicalBytes != 1280 {
		t.Fatalf("merged first storage = %+v", got.Storage[0])
	}
	if got.Storage[0].VisibleObjectCount != 2 ||
		got.Storage[0].RecycleObjectCount != 0 ||
		got.Storage[0].OrphanObjectCount != 1 {
		t.Fatalf("merged diagnostics = %+v", got.Storage[0])
	}
	if !got.Storage[0].IsLegacyProvider || got.Storage[0].SourceProvider != "MINIO" {
		t.Fatalf("legacy provider metadata = %+v", got.Storage[0])
	}
	if !got.Storage[0].MatchedConfig || got.Storage[0].ProviderLabel != "Local MinIO" {
		t.Fatalf("provider metadata = %+v", got.Storage[0])
	}
	if got.ProbeSummary.Total != 3 || got.ProbeSummary.OK != 3 {
		t.Fatalf("probe summary = %+v", got.ProbeSummary)
	}
	if len(got.Probes) != 3 || got.Probes[0].Key != "object-storage:local-minio" {
		t.Fatalf("probes = %+v", got.Probes)
	}
}

func TestResourceMonitorSnapshotWithLibraryScope(t *testing.T) {
	repo := &fakeResourceMonitorRepository{
		rows: []domain.StorageDistributionRow{
			{
				Provider:      "local-minio",
				Bucket:        "default",
				ObjectCount:   1,
				FileRefCount:  1,
				PhysicalBytes: 512,
			},
		},
	}

	uc := NewResourceMonitorUseCase(repo, nil, storage.NewStorageRegistry())
	_, err := uc.Snapshot(
		context.Background(),
		actor.Actor{ID: "42", Kind: actor.KindUser},
		ResourceMonitorSnapshotOptions{LibraryID: 7},
	)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if repo.got != 42 || repo.gotLibrary != 7 {
		t.Fatalf("scope = owner %d library %d, want owner 42 library 7", repo.got, repo.gotLibrary)
	}
}

func TestResourceMonitorDistributionOmitsProbes(t *testing.T) {
	repo := &fakeResourceMonitorRepository{
		rows: []domain.StorageDistributionRow{
			{
				Provider:      "local-minio",
				Bucket:        "default",
				ObjectCount:   1,
				FileRefCount:  1,
				PhysicalBytes: 512,
			},
		},
		ping: errors.New("postgres should not be probed"),
	}
	uc := NewResourceMonitorUseCase(repo, &fakeRedisProbeRepository{err: errors.New("redis should not be probed")}, nil)

	got, err := uc.Distribution(context.Background(), actor.Actor{ID: "42", Kind: actor.KindUser})
	if err != nil {
		t.Fatalf("Distribution() error = %v", err)
	}
	if got.Summary.PhysicalBytes != 512 || len(got.Storage) != 1 {
		t.Fatalf("distribution = %+v", got)
	}
	if got.ProbeSummary.Total != 0 || len(got.Probes) != 0 {
		t.Fatalf("probes should be omitted, got summary %+v probes %+v", got.ProbeSummary, got.Probes)
	}
}

func TestResourceMonitorProbesOmitDistribution(t *testing.T) {
	repo := &fakeResourceMonitorRepository{
		rows: []domain.StorageDistributionRow{
			{
				Provider:      "local-minio",
				Bucket:        "default",
				ObjectCount:   1,
				FileRefCount:  1,
				PhysicalBytes: 512,
			},
		},
	}
	uc := NewResourceMonitorUseCase(repo, &fakeRedisProbeRepository{}, nil)

	got, err := uc.Probes(context.Background(), actor.Actor{ID: "42", Kind: actor.KindUser})
	if err != nil {
		t.Fatalf("Probes() error = %v", err)
	}
	if got.Summary.PhysicalBytes != 0 || len(got.Storage) != 0 {
		t.Fatalf("distribution should be omitted, got summary %+v storage %+v", got.Summary, got.Storage)
	}
	if got.ProbeSummary.Total != 2 || got.ProbeSummary.OK != 2 {
		t.Fatalf("probe summary = %+v", got.ProbeSummary)
	}
}

func TestResourceMonitorBreakdown(t *testing.T) {
	repo := &fakeResourceMonitorRepository{
		libraryRows: []domain.BreakdownLibraryRow{
			{
				LibraryID:             7,
				LibraryName:           "文档",
				ObjectCount:           4,
				FileRefCount:          6,
				PhysicalBytes:         2048,
				ReferencedBytes:       3072,
				VisibleObjectCount:    3,
				VisibleFileRefCount:   4,
				VisibleBytes:          1536,
				RecycleObjectCount:    1,
				RecycleFileRefCount:   2,
				RecycleBytes:          512,
				ArchiveDirectoryCount: 2,
				MultiRefObjectCount:   1,
				MultiRefPhysicalBytes: 512,
				TopProvider:           "local-minio",
				TopBucket:             "default",
			},
			{
				LibraryID:         8,
				LibraryName:       "素材",
				ObjectCount:       1,
				FileRefCount:      0,
				PhysicalBytes:     256,
				OrphanObjectCount: 1,
				OrphanBytes:       256,
				TopProvider:       "win-minio",
				TopBucket:         "default",
			},
			{
				LibraryID:   9,
				LibraryName: "空资料库",
			},
		},
		categoryRows: []domain.BreakdownCategoryRow{
			{
				Key:                   "COMIC",
				BuiltInType:           "COMIC",
				ObjectCount:           3,
				FileRefCount:          4,
				PhysicalBytes:         1536,
				ReferencedBytes:       2048,
				ArchiveDirectoryCount: 1,
				VisibleBytes:          1536,
			},
			{
				Key:               "UNCLASSIFIED",
				ObjectCount:       1,
				PhysicalBytes:     256,
				OrphanBytes:       256,
				OrphanObjectCount: 1,
			},
		},
	}
	uc := NewResourceMonitorUseCase(repo, nil, nil)

	got, err := uc.Breakdown(
		context.Background(),
		actor.Actor{ID: "42", Kind: actor.KindUser},
		ResourceMonitorSnapshotOptions{LibraryID: 7},
	)
	if err != nil {
		t.Fatalf("Breakdown() error = %v", err)
	}
	if repo.got != 42 || repo.gotLibrary != 7 {
		t.Fatalf("scope = owner %d library %d, want owner 42 library 7", repo.got, repo.gotLibrary)
	}
	if got.Summary.LibraryCount != 2 ||
		got.Summary.PhysicalBytes != 2304 ||
		got.Summary.ReferencedBytes != 3072 ||
		got.Summary.ArchiveDirectoryCount != 2 ||
		got.Summary.MultiRefObjectCount != 1 {
		t.Fatalf("summary = %+v", got.Summary)
	}
	if len(got.Libraries) != 2 || got.Libraries[0].Percent != 88.9 {
		t.Fatalf("libraries = %+v", got.Libraries)
	}
	if len(got.Categories) != 2 || got.Categories[0].Key != "COMIC" || got.Categories[0].Label != "漫画" {
		t.Fatalf("categories = %+v", got.Categories)
	}
	if len(got.Statuses) != 3 || got.Statuses[0].Key != "visible" || got.Statuses[0].Percent != 66.7 {
		t.Fatalf("statuses = %+v", got.Statuses)
	}
	if len(got.Anomalies) != 3 || got.Anomalies[0].Key != "top-recycle-library" {
		t.Fatalf("anomalies = %+v", got.Anomalies)
	}
}

func TestResourceMonitorBreakdownReturnsPartialOnCategoryError(t *testing.T) {
	repo := &fakeResourceMonitorRepository{
		libraryRows: []domain.BreakdownLibraryRow{
			{
				LibraryID:       7,
				LibraryName:     "文档",
				ObjectCount:     1,
				FileRefCount:    1,
				PhysicalBytes:   512,
				ReferencedBytes: 512,
			},
		},
		categoryErr: errors.New("category query failed"),
	}
	uc := NewResourceMonitorUseCase(repo, nil, nil)

	got, err := uc.Breakdown(context.Background(), actor.Actor{ID: "42", Kind: actor.KindUser})
	if err != nil {
		t.Fatalf("Breakdown() error = %v", err)
	}
	if got.BreakdownError == "" {
		t.Fatalf("expected breakdown error, got %+v", got)
	}
	if got.Summary.PhysicalBytes != 512 || len(got.Libraries) != 1 || len(got.Categories) != 0 {
		t.Fatalf("partial breakdown = %+v", got)
	}
}

func TestResourceMonitorDashboardBuildsCollectionFileTypeMatrix(t *testing.T) {
	repo := &fakeResourceMonitorRepository{
		libraryRows: []domain.BreakdownLibraryRow{
			{
				LibraryID:       7,
				LibraryName:     "声库",
				ObjectCount:     4,
				FileRefCount:    5,
				PhysicalBytes:   3072,
				ReferencedBytes: 4096,
				VisibleBytes:    3072,
			},
		},
		matrixRows: []domain.DashboardMatrixRow{
			{
				CollectionKey:         "ASMR",
				CollectionBuiltInType: "ASMR",
				FileTypeKey:           "video",
				ObjectCount:           2,
				FileRefCount:          2,
				PhysicalBytes:         2048,
				ReferencedBytes:       2048,
			},
			{
				CollectionKey:         "ASMR",
				CollectionBuiltInType: "ASMR",
				FileTypeKey:           "image",
				ObjectCount:           1,
				FileRefCount:          2,
				PhysicalBytes:         512,
				ReferencedBytes:       1024,
			},
			{
				CollectionKey:   "DEF",
				FileTypeKey:     "text",
				ObjectCount:     1,
				FileRefCount:    1,
				PhysicalBytes:   512,
				ReferencedBytes: 1024,
			},
		},
	}
	uc := NewResourceMonitorUseCase(repo, nil, nil)

	got, err := uc.Dashboard(
		context.Background(),
		actor.Actor{ID: "42", Kind: actor.KindUser},
		ResourceMonitorSnapshotOptions{LibraryID: 7},
	)
	if err != nil {
		t.Fatalf("Dashboard() error = %v", err)
	}
	if repo.got != 42 || repo.gotLibrary != 7 {
		t.Fatalf("scope = owner %d library %d, want owner 42 library 7", repo.got, repo.gotLibrary)
	}
	if got.Summary.PhysicalBytes != 3072 || got.Summary.ReferencedBytes != 4096 {
		t.Fatalf("summary = %+v", got.Summary)
	}
	if len(got.Collections) != 2 || got.Collections[0].Key != "ASMR" || got.Collections[0].PhysicalBytes != 2560 {
		t.Fatalf("collections = %+v", got.Collections)
	}
	if len(got.FileTypes) != 3 || got.FileTypes[0].Key != "video" || got.FileTypes[0].Percent != 66.7 {
		t.Fatalf("fileTypes = %+v", got.FileTypes)
	}
	if len(got.CollectionFileTypeMatrix) != 3 ||
		got.CollectionFileTypeMatrix[0].CollectionKey != "ASMR" ||
		got.CollectionFileTypeMatrix[0].FileTypeKey != "video" ||
		got.CollectionFileTypeMatrix[0].PercentOfCollection != 80 {
		t.Fatalf("matrix = %+v", got.CollectionFileTypeMatrix)
	}
}

func TestResourceMonitorDashboardReturnsPartialOnMatrixError(t *testing.T) {
	repo := &fakeResourceMonitorRepository{
		libraryRows: []domain.BreakdownLibraryRow{
			{
				LibraryID:       7,
				LibraryName:     "文档",
				ObjectCount:     1,
				FileRefCount:    1,
				PhysicalBytes:   512,
				ReferencedBytes: 512,
			},
		},
		matrixErr: errors.New("matrix query failed"),
	}
	uc := NewResourceMonitorUseCase(repo, nil, nil)

	got, err := uc.Dashboard(context.Background(), actor.Actor{ID: "42", Kind: actor.KindUser})
	if err != nil {
		t.Fatalf("Dashboard() error = %v", err)
	}
	if got.DashboardError == "" {
		t.Fatalf("expected dashboard error, got %+v", got)
	}
	if got.Summary.PhysicalBytes != 512 || len(got.Libraries) != 1 || len(got.CollectionFileTypeMatrix) != 0 {
		t.Fatalf("partial dashboard = %+v", got)
	}
}

func TestResourceMonitorCaptureSample(t *testing.T) {
	repo := &fakeResourceMonitorRepository{
		rows: []domain.StorageDistributionRow{
			{
				Provider:            "local-minio",
				Bucket:              "default",
				ObjectCount:         2,
				FileRefCount:        2,
				PhysicalBytes:       1024,
				VisibleObjectCount:  1,
				VisibleFileRefCount: 1,
				VisibleBytes:        512,
				RecycleObjectCount:  1,
				RecycleFileRefCount: 1,
				RecycleBytes:        512,
			},
		},
	}

	uc := NewResourceMonitorUseCase(repo, &fakeRedisProbeRepository{}, storage.NewStorageRegistry())
	got, err := uc.CaptureSample(
		context.Background(),
		actor.Actor{ID: "42", Kind: actor.KindUser},
		ResourceMonitorSnapshotOptions{LibraryID: 7},
	)
	if err != nil {
		t.Fatalf("CaptureSample() error = %v", err)
	}
	if got.ID != 1 || repo.saved.ActorID != "42" || repo.saved.Scope != "library" || repo.saved.LibraryID != 7 {
		t.Fatalf("sample identity = got %+v saved %+v", got, repo.saved)
	}
	if repo.saved.PhysicalBytes != 1024 || repo.saved.RecycleBytes != 512 || repo.saved.ProbeTotal != 2 {
		t.Fatalf("sample metrics = %+v", repo.saved)
	}
	if !strings.Contains(repo.saved.SnapshotJSON, `"physicalBytes":1024`) {
		t.Fatalf("sample snapshot json = %s", repo.saved.SnapshotJSON)
	}
}

func TestResourceMonitorCaptureSampleDryRun(t *testing.T) {
	repo := &fakeResourceMonitorRepository{
		rows: []domain.StorageDistributionRow{
			{
				Provider:      "local-minio",
				Bucket:        "default",
				ObjectCount:   1,
				FileRefCount:  1,
				PhysicalBytes: 512,
			},
		},
	}

	uc := NewResourceMonitorUseCase(repo, nil, storage.NewStorageRegistry())
	got, err := uc.CaptureSample(
		context.Background(),
		actor.Actor{ID: "42", Kind: actor.KindUser},
		ResourceMonitorSnapshotOptions{LibraryID: 7, DryRun: true},
	)
	if err != nil {
		t.Fatalf("CaptureSample(dry-run) error = %v", err)
	}
	if !got.DryRun || got.ID != 0 || repo.saved.ActorID != "" {
		t.Fatalf("dry-run sample = got %+v saved %+v", got, repo.saved)
	}
	if got.Scope != "library" || got.LibraryID != 7 || got.PhysicalBytes != 512 {
		t.Fatalf("dry-run sample metrics = %+v", got)
	}
}

func TestResourceMonitorCaptureSampleRejectsUnownedLibrary(t *testing.T) {
	repo := &fakeResourceMonitorRepository{ownedBy: map[uint64]bool{7: false}}
	uc := NewResourceMonitorUseCase(repo, nil, storage.NewStorageRegistry())

	_, err := uc.CaptureSample(
		context.Background(),
		actor.Actor{ID: "42", Kind: actor.KindUser},
		ResourceMonitorSnapshotOptions{LibraryID: 7},
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("CaptureSample() error = %v, want ErrNotFound", err)
	}
	if repo.gotLibrary != 0 {
		t.Fatalf("distribution query libraryID = %d, want 0 before ownership passes", repo.gotLibrary)
	}
	if repo.saved.ActorID != "" {
		t.Fatalf("unexpected saved sample = %+v", repo.saved)
	}
}

func TestResourceMonitorSnapshotRejectsMissingActor(t *testing.T) {
	uc := NewResourceMonitorUseCase(&fakeResourceMonitorRepository{}, nil, nil)
	_, err := uc.Snapshot(context.Background(), actor.Actor{Kind: actor.KindAnonymous})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Snapshot() error = %v, want ErrInvalidArgument", err)
	}
}

func TestResourceMonitorSnapshotReportsProbeErrors(t *testing.T) {
	repo := &fakeResourceMonitorRepository{ping: errors.New("postgres unavailable")}
	registry := storage.NewStorageRegistry()
	_, err := registry.Reload(&config.StorageConfig{
		Providers: map[string]config.ProviderConfig{
			"remote-minio": {
				Type:     "MINIO",
				Endpoint: "192.168.1.10:9000",
				Bucket:   "default",
				Label:    "Remote MinIO",
			},
		},
		DefaultProvider: "remote-minio",
	}, func(alias string, cfg config.ProviderConfig) (storage.ObjectStorage, func(), error) {
		return fakeObjectStorage{
			bucket: cfg.Bucket,
			err:    errors.New("check bucket exists: connection closed by foreign host"),
		}, func() {}, nil
	})
	if err != nil {
		t.Fatalf("reload registry: %v", err)
	}

	uc := NewResourceMonitorUseCase(repo, &fakeRedisProbeRepository{err: errors.New("redis unavailable")}, registry)
	got, err := uc.Snapshot(context.Background(), actor.Actor{ID: "42", Kind: actor.KindUser})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	if got.ProbeSummary.Total != 3 || got.ProbeSummary.Error != 3 {
		t.Fatalf("probe summary = %+v", got.ProbeSummary)
	}
	for _, probe := range got.Probes {
		if probe.Status != domain.ProbeStatusError || probe.Error == "" {
			t.Fatalf("probe = %+v", probe)
		}
	}
}

func TestResourceMonitorSnapshotReturnsPartialWhenDistributionFails(t *testing.T) {
	repo := &fakeResourceMonitorRepository{
		err: errors.New(
			`dial postgres://user:secret@127.0.0.1:5432/omniflow?password=topsecret ` +
				`access_key=AKIAIOSFODNN7EXAMPLE token:abcdef`,
		),
		ping: errors.New("postgres unavailable password=hidden"),
	}
	uc := NewResourceMonitorUseCase(repo, &fakeRedisProbeRepository{}, nil)
	got, err := uc.Snapshot(context.Background(), actor.Actor{ID: "42", Kind: actor.KindUser})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	if got.DistributionError == "" {
		t.Fatalf("DistributionError is empty")
	}
	for _, leaked := range []string{"secret", "topsecret", "abcdef", "hidden", "AKIAIOSFODNN7EXAMPLE"} {
		if strings.Contains(got.DistributionError, leaked) {
			t.Fatalf("DistributionError leaked %q: %s", leaked, got.DistributionError)
		}
	}
	if len(got.Storage) != 0 {
		t.Fatalf("storage len = %d, want 0", len(got.Storage))
	}
	if got.ProbeSummary.Total != 2 || got.ProbeSummary.Error != 1 || got.ProbeSummary.OK != 1 {
		t.Fatalf("probe summary = %+v", got.ProbeSummary)
	}
}
