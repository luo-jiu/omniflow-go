package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultBaseURL = "http://127.0.0.1:8850"
	envBaseURL     = "OMNIFLOW_BASE_URL"
	envUsername    = "OMNIFLOW_USERNAME"
	envToken       = "OMNIFLOW_TOKEN"
)

var ErrInvalidConfig = errors.New("invalid cli config")

type Session struct {
	BaseURL  string `json:"baseUrl"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

func ConfigFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".omniflow", "cli.json"), nil
}

func LoadSession() (Session, error) {
	configPath, err := ConfigFilePath()
	if err != nil {
		return Session{BaseURL: defaultBaseURL}, err
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Session{BaseURL: defaultBaseURL}, nil
		}
		return Session{BaseURL: defaultBaseURL}, err
	}

	var session Session
	if err := json.Unmarshal(content, &session); err != nil {
		return Session{BaseURL: defaultBaseURL}, fmt.Errorf("%w: parse cli config: %v", ErrInvalidConfig, err)
	}

	session.BaseURL = normalizeBaseURL(session.BaseURL)
	session.Username = strings.TrimSpace(session.Username)
	session.Token = strings.TrimSpace(session.Token)
	return session, nil
}

func SaveSession(session Session) error {
	configPath, err := ConfigFilePath()
	if err != nil {
		return err
	}

	session.BaseURL = normalizeBaseURL(session.BaseURL)
	session.Username = strings.TrimSpace(session.Username)
	session.Token = strings.TrimSpace(session.Token)

	content, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cli config: %w", err)
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("prepare config dir: %w", err)
	}

	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o600); err != nil {
		return fmt.Errorf("write cli config: %w", err)
	}
	return os.Rename(tmpPath, configPath)
}

func ResolveSession(baseURLOverride string) (Session, error) {
	session, err := LoadSession()
	if err != nil {
		if errors.Is(err, ErrInvalidConfig) {
			session = Session{BaseURL: defaultBaseURL}
		} else {
			return Session{}, err
		}
	}

	baseURL := strings.TrimSpace(baseURLOverride)
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv(envBaseURL))
	}
	if baseURL == "" {
		baseURL = session.BaseURL
	}

	username := strings.TrimSpace(os.Getenv(envUsername))
	if username == "" {
		username = session.Username
	}

	token := strings.TrimSpace(os.Getenv(envToken))
	if token == "" {
		token = session.Token
	}

	return Session{
		BaseURL:  normalizeBaseURL(baseURL),
		Username: username,
		Token:    token,
	}, nil
}

func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return defaultBaseURL
	}
	return trimmed
}
