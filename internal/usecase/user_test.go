package usecase

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestResolveAvatarUploadExt(t *testing.T) {
	if got := resolveAvatarUploadExt(".png", "image/jpeg"); got != ".png" {
		t.Fatalf("expected .png, got %s", got)
	}
	if got := resolveAvatarUploadExt("", "image/webp"); got != ".webp" {
		t.Fatalf("expected .webp, got %s", got)
	}
	if got := resolveAvatarUploadExt("", "application/octet-stream"); got != "" {
		t.Fatalf("expected empty ext, got %s", got)
	}
}

func TestResolveAvatarContentType(t *testing.T) {
	if got := resolveAvatarContentType(".jpg"); got != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %s", got)
	}
	if got := resolveAvatarContentType(".avif"); got != "image/avif" {
		t.Fatalf("expected image/avif, got %s", got)
	}
}

func TestBuildAvatarStorageKey(t *testing.T) {
	key := buildAvatarStorageKey(7, ".png")
	if !strings.HasPrefix(key, "user/7/avatar/") {
		t.Fatalf("unexpected key prefix: %s", key)
	}

	monthPath := time.Now().UTC().Format("2006/01")
	pattern := `^user/7/avatar/` + regexp.QuoteMeta(monthPath) + `/[a-f0-9]{32}\.png$`
	if matched := regexp.MustCompile(pattern).MatchString(key); !matched {
		t.Fatalf("unexpected key format: %s", key)
	}
}
