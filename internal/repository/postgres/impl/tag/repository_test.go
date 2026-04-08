package repository

import "testing"

func TestEnabledConversion(t *testing.T) {
	if !toDBEnabled(1) {
		t.Fatalf("expected enabled=1 convert to db true")
	}
	if toDBEnabled(0) {
		t.Fatalf("expected enabled=0 convert to db false")
	}

	if got := toAPIEnabled(true); got != 1 {
		t.Fatalf("expected db true convert to api 1, got %d", got)
	}
	if got := toAPIEnabled(false); got != 0 {
		t.Fatalf("expected db false convert to api 0, got %d", got)
	}
}
