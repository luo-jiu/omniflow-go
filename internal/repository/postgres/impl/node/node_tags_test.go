package repository

import "testing"

func TestNormalizeRelationTagIDs(t *testing.T) {
	got := normalizeRelationTagIDs([]uint64{0, 7, 7, 2, 0, 9})
	want := []uint64{7, 2, 9}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d; got %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tagIDs[%d] = %d, want %d; got %#v", i, got[i], want[i], got)
		}
	}
}
