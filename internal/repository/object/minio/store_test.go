package repository

import (
	"errors"
	"testing"

	"github.com/minio/minio-go/v7"
)

func TestNormalizeMinIOEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		raw       string
		useSSL    bool
		wantHost  string
		wantSSL   bool
		wantError bool
	}{
		{
			name:     "plain host port",
			raw:      "localhost:9000",
			useSSL:   false,
			wantHost: "localhost:9000",
			wantSSL:  false,
		},
		{
			name:     "http url",
			raw:      "http://localhost:9000",
			useSSL:   true,
			wantHost: "localhost:9000",
			wantSSL:  false,
		},
		{
			name:     "https url",
			raw:      "https://storage.example.com:9000",
			useSSL:   false,
			wantHost: "storage.example.com:9000",
			wantSSL:  true,
		},
		{
			name:      "empty endpoint",
			raw:       " ",
			useSSL:    false,
			wantError: true,
		},
		{
			name:      "invalid endpoint url",
			raw:       "http://%",
			useSSL:    false,
			wantError: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotHost, gotSSL, err := normalizeMinIOEndpoint(tc.raw, tc.useSSL)
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotHost != tc.wantHost {
				t.Fatalf("host mismatch: got %q, want %q", gotHost, tc.wantHost)
			}
			if gotSSL != tc.wantSSL {
				t.Fatalf("ssl mismatch: got %v, want %v", gotSSL, tc.wantSSL)
			}
		})
	}
}

func TestIsBucketAlreadyExistsError(t *testing.T) {
	t.Parallel()

	if !isBucketAlreadyExistsError(minio.ErrorResponse{Code: "BucketAlreadyOwnedByYou"}) {
		t.Fatalf("expected BucketAlreadyOwnedByYou to be treated as already-exists")
	}
	if !isBucketAlreadyExistsError(minio.ErrorResponse{Code: "BucketAlreadyExists"}) {
		t.Fatalf("expected BucketAlreadyExists to be treated as already-exists")
	}
	if isBucketAlreadyExistsError(minio.ErrorResponse{Code: "AccessDenied"}) {
		t.Fatalf("did not expect AccessDenied to be treated as already-exists")
	}
	if isBucketAlreadyExistsError(errors.New("network error")) {
		t.Fatalf("did not expect generic error to be treated as already-exists")
	}
}
