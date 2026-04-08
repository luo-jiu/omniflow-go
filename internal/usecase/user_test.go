package usecase

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestResolveAvatarUploadExtByMIME(t *testing.T) {
	if got := resolveAvatarUploadExtByMIME("image/webp"); got != ".webp" {
		t.Fatalf("expected .webp, got %s", got)
	}
	if got := resolveAvatarUploadExtByMIME("image/x-icon"); got != ".ico" {
		t.Fatalf("expected .ico, got %s", got)
	}
	if got := resolveAvatarUploadExtByMIME("application/octet-stream"); got != "" {
		t.Fatalf("expected empty ext, got %s", got)
	}
}

func TestNormalizeAvatarUploadDetectImageMIME(t *testing.T) {
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0, 'I', 'H', 'D', 'R'}
	reader, ext, contentType, err := normalizeAvatarUpload(bytes.NewReader(pngHeader), "avatar.bin", "application/octet-stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ext != ".png" {
		t.Fatalf("expected .png, got %s", ext)
	}
	if contentType != "image/png" {
		t.Fatalf("expected image/png, got %s", contentType)
	}

	consumed, err := reader.Read(make([]byte, 8))
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if consumed == 0 {
		t.Fatalf("expected readable content, got 0 bytes")
	}
}

func TestNormalizeAvatarUploadRejectSVG(t *testing.T) {
	_, _, _, err := normalizeAvatarUpload(bytes.NewReader([]byte("<svg/>")), "avatar.svg", "image/svg+xml")
	if err == nil {
		t.Fatalf("expected error for svg avatar")
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
