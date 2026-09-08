package handler

import (
	"encoding/json"
	"testing"
	"time"

	domainnode "omniflow-go/internal/domain/node"
)

func TestNodeDetailResponseIncludesTimestamps(t *testing.T) {
	createdAt := time.Date(2026, time.August, 25, 12, 30, 15, 0, time.UTC)
	updatedAt := createdAt.Add(90 * time.Minute)
	response := newNodeDetailResponse(domainnode.Node{
		ID:        42,
		Name:      "notes",
		Type:      domainnode.TypeFile,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	})

	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal node detail response: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal node detail response: %v", err)
	}
	if decoded["createdAt"] != createdAt.Format(time.RFC3339) {
		t.Fatalf("createdAt = %v, want %s", decoded["createdAt"], createdAt.Format(time.RFC3339))
	}
	if decoded["updatedAt"] != updatedAt.Format(time.RFC3339) {
		t.Fatalf("updatedAt = %v, want %s", decoded["updatedAt"], updatedAt.Format(time.RFC3339))
	}
}
