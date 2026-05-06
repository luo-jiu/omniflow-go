package usecase

import "testing"

func TestNormalizeTagClassificationInfersResourceKind(t *testing.T) {
	scope, dimension, resourceKind, err := normalizeTagClassification("", "creator", "", "COMIC")
	if err != nil {
		t.Fatalf("normalizeTagClassification() error = %v", err)
	}
	if scope != "resource" {
		t.Fatalf("scope = %q, want resource", scope)
	}
	if dimension != "creator" {
		t.Fatalf("dimension = %q, want creator", dimension)
	}
	if resourceKind == nil || *resourceKind != "comic" {
		t.Fatalf("resourceKind = %v, want comic", resourceKind)
	}
}

func TestNormalizeTagClassificationForFileTab(t *testing.T) {
	scope, dimension, resourceKind, err := normalizeTagClassification("resource", "", "audio", "FILE_TAB")
	if err != nil {
		t.Fatalf("normalizeTagClassification() error = %v", err)
	}
	if scope != "ui" {
		t.Fatalf("scope = %q, want ui", scope)
	}
	if dimension != "custom" {
		t.Fatalf("dimension = %q, want custom", dimension)
	}
	if resourceKind != nil {
		t.Fatalf("resourceKind = %v, want nil", resourceKind)
	}
}

func TestExtractViewMetaTagIDs(t *testing.T) {
	got, ok, err := extractViewMetaTagIDs(`{"tagIds":[3,0,3,5],"other":true}`)
	if err != nil {
		t.Fatalf("extractViewMetaTagIDs() error = %v", err)
	}
	if !ok {
		t.Fatalf("extractViewMetaTagIDs() ok = false, want true")
	}
	if len(got) != 2 || got[0] != 3 || got[1] != 5 {
		t.Fatalf("tag ids = %#v, want [3 5]", got)
	}
}

func TestExtractViewMetaTagIDsMissing(t *testing.T) {
	got, ok, err := extractViewMetaTagIDs(`{"title":"demo"}`)
	if err != nil {
		t.Fatalf("extractViewMetaTagIDs() error = %v", err)
	}
	if ok {
		t.Fatalf("extractViewMetaTagIDs() ok = true, want false")
	}
	if got != nil {
		t.Fatalf("tag ids = %#v, want nil", got)
	}
}
