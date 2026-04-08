package bootstrap

import (
	"log/slog"
	"os"
	"strings"

	"omniflow-go/internal/config"
)

func NewLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo

	switch strings.ToLower(cfg.Server.Mode) {
	case "debug":
		level = slog.LevelDebug
	case "release":
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler).With(
		"service", cfg.App.Name,
		"env", cfg.App.Env,
		"version", cfg.App.Version,
	)
}
