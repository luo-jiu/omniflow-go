package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSessionFallsBackWhenConfigIsInvalid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OMNIFLOW_BASE_URL", "")
	t.Setenv("OMNIFLOW_USERNAME", "")
	t.Setenv("OMNIFLOW_TOKEN", "")

	configDir := filepath.Join(home, ".omniflow")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "cli.json")
	if err := os.WriteFile(configPath, []byte("{invalid json"), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	session, err := ResolveSession("")
	if err != nil {
		t.Fatalf("resolve session: %v", err)
	}
	if session.BaseURL != defaultBaseURL {
		t.Fatalf("unexpected base url: got=%s want=%s", session.BaseURL, defaultBaseURL)
	}
	if session.Username != "" {
		t.Fatalf("expected empty username, got=%q", session.Username)
	}
	if session.Token != "" {
		t.Fatalf("expected empty token, got=%q", session.Token)
	}
}
