package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCompletionReceiptRoundTrip(t *testing.T) {
	dsn := os.Getenv("OMNIFLOW_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OMNIFLOW_TEST_POSTGRES_DSN is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	repo := NewUploadSessionRepository(tx)
	ctx := context.Background()
	now := time.Now().UTC()
	uploadID := "test-upload-" + uuid.NewString()
	operationID := "test-operation-" + uuid.NewString()
	_, err = repo.Create(ctx, CreateInput{
		ID:              uploadID,
		LibraryID:       2,
		ActorID:         "integration-test-actor",
		StorageKey:      "test/" + uploadID,
		FileName:        "file.txt",
		FileSize:        4,
		ContentType:     "text/plain",
		StorageProvider: "local-minio",
		Mode:            "single",
		PartSize:        4,
		ExpiresAt:       now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if ok, err := repo.ClaimOperation(ctx, uploadID, operationID, now.Add(time.Hour)); err != nil || !ok {
		t.Fatalf("claim operation: ok=%v err=%v", ok, err)
	}
	if ok, err := repo.ClaimOperation(ctx, uploadID, "other-operation", now.Add(time.Hour)); err != nil || ok {
		t.Fatalf("second operation must not replace the first claim: ok=%v err=%v", ok, err)
	}
	if ok, err := repo.UpdateExpiresAt(ctx, uploadID, now.Add(2*time.Hour)); err != nil || ok {
		t.Fatalf("renew must not mutate a claimed session: ok=%v err=%v", ok, err)
	}
	if ok, err := repo.ClaimExpiredForCleanup(
		ctx,
		uploadID,
		now,
		"internal:janitor:"+uploadID,
		now.Add(time.Minute),
	); err != nil || ok {
		t.Fatalf("janitor must not claim a session with a renewed future lease: ok=%v err=%v", ok, err)
	}
	if ok, err := repo.MarkCommitted(
		ctx,
		uploadID,
		operationID,
		42,
		`{"id":42,"name":"file.txt","type":"file","libraryId":2}`,
		now,
		now.Add(7*24*time.Hour),
	); err != nil || !ok {
		t.Fatalf("mark committed: ok=%v err=%v", ok, err)
	}

	receipt, err := repo.GetByOperationIDAndActor(ctx, "integration-test-actor", operationID)
	if err != nil {
		t.Fatalf("read receipt: %v", err)
	}
	if receipt.CompletedNodeID != 42 || receipt.CompletionResult == "" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	if ok, err := repo.UpdateExpiresAt(ctx, uploadID, now.Add(2*time.Hour)); err != nil || ok {
		t.Fatalf("renew must not mutate a committed receipt: ok=%v err=%v", ok, err)
	}
}
