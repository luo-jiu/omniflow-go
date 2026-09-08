package repository

import (
	"testing"
	"time"

	pgmodel "omniflow-go/internal/repository/postgres/model"
)

func TestToDomainNodeModelPreservesTimestamps(t *testing.T) {
	createdAt := time.Date(2026, time.August, 25, 12, 30, 15, 0, time.UTC)
	updatedAt := createdAt.Add(90 * time.Minute)

	result := toDomainNodeModel(&pgmodel.Node{
		ID:        42,
		LibraryID: 3,
		Name:      "notes",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	})

	if !result.CreatedAt.Equal(createdAt) {
		t.Fatalf("createdAt = %s, want %s", result.CreatedAt, createdAt)
	}
	if !result.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("updatedAt = %s, want %s", result.UpdatedAt, updatedAt)
	}
}
