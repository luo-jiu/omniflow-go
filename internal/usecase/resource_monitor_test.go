package usecase

import (
	"context"
	"errors"
	"testing"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/config"
	domain "omniflow-go/internal/domain/resourcemonitor"
	"omniflow-go/internal/storage"
)

type fakeResourceMonitorRepository struct {
	rows []domain.StorageDistributionRow
	err  error
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
		return nil, func() {}, nil
	})
	if err != nil {
		t.Fatalf("reload registry: %v", err)
	}

	uc := NewResourceMonitorUseCase(repo, registry)
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
}

func TestResourceMonitorSnapshotRejectsMissingActor(t *testing.T) {
	uc := NewResourceMonitorUseCase(&fakeResourceMonitorRepository{}, nil)
	_, err := uc.Snapshot(context.Background(), actor.Actor{Kind: actor.KindAnonymous})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Snapshot() error = %v, want ErrInvalidArgument", err)
	}
}
