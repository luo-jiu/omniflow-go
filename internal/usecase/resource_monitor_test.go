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
	rows []domain.StorageDistributionRow
	err  error
	ping error
	got  uint64
}

func (r *fakeResourceMonitorRepository) CountStorageDistribution(
	_ context.Context,
	ownerUserID uint64,
) ([]domain.StorageDistributionRow, error) {
	r.got = ownerUserID
	if r.err != nil {
		return nil, r.err
	}
	return r.rows, nil
}

func (r *fakeResourceMonitorRepository) Ping(_ context.Context) error {
	return r.ping
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
				Provider:      "local-minio",
				Bucket:        "default",
				ObjectCount:   2,
				FileRefCount:  3,
				PhysicalBytes: 1024,
			},
			{
				Provider:      "unknown-provider",
				Bucket:        "legacy",
				ObjectCount:   1,
				FileRefCount:  1,
				PhysicalBytes: 512,
			},
			{
				Provider:      "MINIO",
				Bucket:        "default",
				ObjectCount:   1,
				FileRefCount:  2,
				PhysicalBytes: 256,
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
	if got.Summary.ProviderCount != 2 || got.Summary.BucketCount != 2 {
		t.Fatalf("summary provider/bucket = %d/%d, want 2/2", got.Summary.ProviderCount, got.Summary.BucketCount)
	}
	if got.Summary.ObjectCount != 4 || got.Summary.FileRefCount != 6 || got.Summary.PhysicalBytes != 1792 {
		t.Fatalf("summary counts = %+v", got.Summary)
	}
	if got.Summary.UnmatchedCount != 1 {
		t.Fatalf("unmatched = %d, want 1", got.Summary.UnmatchedCount)
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
