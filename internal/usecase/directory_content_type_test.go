package usecase

import (
	"bytes"
	"io"
	"testing"
)

func TestResolveUploadContentType_WebpSniff(t *testing.T) {
	webp := []byte{
		'R', 'I', 'F', 'F', 0x2A, 0x00, 0x00, 0x00,
		'W', 'E', 'B', 'P', 'V', 'P', '8', ' ',
		0x01, 0x02, 0x03, 0x04, 0x05,
	}
	reader, contentType, err := resolveUploadContentType(bytes.NewReader(webp), "", ".webp")
	if err != nil {
		t.Fatalf("resolveUploadContentType error: %v", err)
	}
	if contentType != "image/webp" {
		t.Fatalf("unexpected contentType: got %q, want %q", contentType, "image/webp")
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read replay reader error: %v", err)
	}
	if !bytes.Equal(data, webp) {
		t.Fatalf("replayed stream mismatch")
	}
}

func TestResolveUploadContentType_DeclaredTypePreferred(t *testing.T) {
	payload := []byte("hello")
	reader, contentType, err := resolveUploadContentType(bytes.NewReader(payload), "audio/mpeg", ".mp3")
	if err != nil {
		t.Fatalf("resolveUploadContentType error: %v", err)
	}
	if contentType != "audio/mpeg" {
		t.Fatalf("unexpected contentType: got %q, want %q", contentType, "audio/mpeg")
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read replay reader error: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("replayed stream mismatch")
	}
}

func TestResolveUploadContentType_ExtFallback(t *testing.T) {
	payload := []byte{}
	_, contentType, err := resolveUploadContentType(bytes.NewReader(payload), "", ".jpg")
	if err != nil {
		t.Fatalf("resolveUploadContentType error: %v", err)
	}
	if contentType == defaultUploadContentType {
		t.Fatalf("expected extension fallback, got %q", contentType)
	}
}

func TestExtractUploadBaseName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "unix path",
			input:    "/tmp/漫画/[C.R] name 01.webp",
			expected: "[C.R] name 01.webp",
		},
		{
			name:     "windows path",
			input:    `C:\Users\test\ASMR\A"B"C.mp3`,
			expected: `A"B"C.mp3`,
		},
		{
			name:     "plain file name",
			input:    "纯字符串文件名.flac",
			expected: "纯字符串文件名.flac",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := extractUploadBaseName(tc.input)
			if got != tc.expected {
				t.Fatalf("unexpected base name: got %q, want %q", got, tc.expected)
			}
		})
	}
}
