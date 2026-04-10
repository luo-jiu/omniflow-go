package bootstrap

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"omniflow-go/internal/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

func NewLogger(cfg *config.Config) *slog.Logger {
	level := parseLogLevel(cfg.Log.Level)
	warnings := make([]string, 0, 2)
	writers := make([]io.Writer, 0, 2)

	if cfg.Log.Console.Enabled {
		writers = append(writers, os.Stdout)
	}

	if cfg.Log.File.Enabled {
		logFilePath := strings.TrimSpace(cfg.Log.File.Path)
		if logFilePath == "" {
			warnings = append(warnings, "log.file.enabled=true but log.file.path is empty, file output skipped")
		} else {
			if err := os.MkdirAll(filepath.Dir(logFilePath), 0o755); err != nil {
				warnings = append(warnings, "create log directory failed: "+err.Error())
			} else {
				writers = append(writers, &lumberjack.Logger{
					Filename:   logFilePath,
					MaxSize:    cfg.Log.File.MaxSizeMB,
					MaxBackups: cfg.Log.File.MaxBackups,
					MaxAge:     cfg.Log.File.MaxAgeDays,
					Compress:   cfg.Log.File.Compress,
					LocalTime:  cfg.Log.File.LocalTime,
				})
			}
		}
	}

	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
		warnings = append(warnings, "no log output enabled, fallback to stdout")
	}

	output := io.MultiWriter(writers...)
	handlerOptions := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.Log.AddSource,
	}

	var handler slog.Handler
	if strings.EqualFold(cfg.Log.Format, "json") {
		handler = slog.NewJSONHandler(output, handlerOptions)
	} else {
		handler = slog.NewTextHandler(output, handlerOptions)
	}

	logger := slog.New(handler).With(
		"service", cfg.App.Name,
		"env", cfg.App.Env,
		"version", cfg.App.Version,
	)

	for _, warning := range warnings {
		logger.Warn("logger config warning", "detail", warning)
	}
	logger.Info("logger initialized",
		"level", strings.ToLower(strings.TrimSpace(cfg.Log.Level)),
		"format", strings.ToLower(strings.TrimSpace(cfg.Log.Format)),
		"console_enabled", cfg.Log.Console.Enabled,
		"file_enabled", cfg.Log.File.Enabled,
		"file_path", strings.TrimSpace(cfg.Log.File.Path),
	)
	slog.SetDefault(logger)
	return logger
}

func parseLogLevel(raw string) slog.Leveler {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info":
		fallthrough
	default:
		return slog.LevelInfo
	}
}
