package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App      App      `yaml:"app"`
	Server   Server   `yaml:"server"`
	Log      Log      `yaml:"log"`
	Database Database `yaml:"database"`
	Redis    Redis    `yaml:"redis"`
	Storage  Storage  `yaml:"storage"`
	MinIO    MinIO    `yaml:"minio"`
}

type App struct {
	Name    string `yaml:"name"`
	Env     string `yaml:"env"`
	Version string `yaml:"version"`
}

type Server struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Mode            string        `yaml:"mode"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type Log struct {
	Level     string     `yaml:"level"`
	Format    string     `yaml:"format"`
	AddSource bool       `yaml:"add_source"`
	Console   LogConsole `yaml:"console"`
	File      LogFile    `yaml:"file"`
}

type LogConsole struct {
	Enabled bool `yaml:"enabled"`
	Color   bool `yaml:"color"`
}

type LogFile struct {
	Enabled    bool   `yaml:"enabled"`
	Path       string `yaml:"path"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
	Compress   bool   `yaml:"compress"`
	LocalTime  bool   `yaml:"local_time"`
}

type Database struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	LogLevel        string        `yaml:"log_level"`
	DebugSQL        bool          `yaml:"debug_sql"`
}

type Redis struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type Storage struct {
	Provider string `yaml:"provider"`
}

type MinIO struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	UseSSL    bool   `yaml:"use_ssl"`
	Bucket    string `yaml:"bucket"`
}

func Load(path string) (*Config, error) {
	cfg := defaultConfig()

	if path == "" {
		if err := cfg.applyDefaults(); err != nil {
			return nil, fmt.Errorf("apply config defaults: %w", err)
		}
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if applyErr := cfg.applyDefaults(); applyErr != nil {
				return nil, fmt.Errorf("apply config defaults: %w", applyErr)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.applyDefaults(); err != nil {
		return nil, fmt.Errorf("apply config defaults: %w", err)
	}
	return cfg, nil
}

func (s Server) Address() string {
	return net.JoinHostPort(s.Host, fmt.Sprintf("%d", s.Port))
}

func defaultConfig() *Config {
	return &Config{
		App: App{
			Name:    "omniflow-go",
			Env:     "local",
			Version: "0.1.0",
		},
		Server: Server{
			Host:            "0.0.0.0",
			Port:            8850,
			Mode:            "debug",
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    15 * time.Second,
			IdleTimeout:     60 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		Log: Log{
			Console: LogConsole{
				Enabled: true,
				Color:   true,
			},
			File: LogFile{
				Enabled:    false,
				MaxSizeMB:  100,
				MaxBackups: 10,
				MaxAgeDays: 30,
				Compress:   true,
				LocalTime:  true,
			},
		},
		Database: Database{
			DSN:             "postgres://postgres:123456@127.0.0.1:5432/omniflow?sslmode=disable",
			MaxOpenConns:    20,
			MaxIdleConns:    10,
			ConnMaxLifetime: 30 * time.Minute,
			LogLevel:        "warn",
		},
		Redis: Redis{
			Addr: "127.0.0.1:6379",
			DB:   0,
		},
		Storage: Storage{
			Provider: "minio",
		},
		MinIO: MinIO{
			Endpoint:  "localhost:9000",
			AccessKey: "admin",
			SecretKey: "admin123",
			UseSSL:    false,
			Bucket:    "my-bucket",
		},
	}
}

func (c *Config) applyDefaults() error {
	if c.App.Name == "" {
		c.App.Name = "omniflow-go"
	}
	if c.App.Env == "" {
		c.App.Env = "local"
	}
	if c.App.Version == "" {
		c.App.Version = "0.1.0"
	}

	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8850
	}
	if c.Server.Mode == "" {
		c.Server.Mode = "debug"
	}
	normalizedMode, err := normalizeServerMode(c.Server.Mode)
	if err != nil {
		return err
	}
	c.Server.Mode = normalizedMode
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 10 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 15 * time.Second
	}
	if c.Server.IdleTimeout == 0 {
		c.Server.IdleTimeout = 60 * time.Second
	}
	if c.Server.ShutdownTimeout == 0 {
		c.Server.ShutdownTimeout = 10 * time.Second
	}

	mode := c.Server.Mode
	normalizedLevel, err := normalizeLogLevel(c.Log.Level, mode)
	if err != nil {
		return err
	}
	c.Log.Level = normalizedLevel

	normalizedFormat, err := normalizeLogFormat(c.Log.Format, mode)
	if err != nil {
		return err
	}
	c.Log.Format = normalizedFormat
	if c.Log.File.MaxSizeMB <= 0 {
		c.Log.File.MaxSizeMB = 100
	}
	if c.Log.File.MaxBackups <= 0 {
		c.Log.File.MaxBackups = 10
	}
	if c.Log.File.MaxAgeDays <= 0 {
		c.Log.File.MaxAgeDays = 30
	}

	if c.Database.MaxOpenConns == 0 {
		c.Database.MaxOpenConns = 20
	}
	if c.Database.MaxIdleConns == 0 {
		c.Database.MaxIdleConns = 10
	}
	if c.Database.ConnMaxLifetime == 0 {
		c.Database.ConnMaxLifetime = 30 * time.Minute
	}
	normalizedDBLevel, err := normalizeDatabaseLogLevel(c.Database.LogLevel)
	if err != nil {
		return err
	}
	if c.Database.DebugSQL {
		// 开启 SQL 调试时，强制提升到 info，确保语句会输出。
		c.Database.LogLevel = "info"
	} else {
		c.Database.LogLevel = normalizedDBLevel
	}

	if c.Storage.Provider == "" {
		c.Storage.Provider = "minio"
	}

	if c.MinIO.Bucket == "" {
		c.MinIO.Bucket = "my-bucket"
	}
	return nil
}

func normalizeServerMode(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "debug", "release", "test":
		return strings.ToLower(strings.TrimSpace(mode)), nil
	default:
		return "", fmt.Errorf("invalid server.mode: %q (supported: debug|release|test)", mode)
	}
}

func normalizeLogLevel(level, mode string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(level))
	switch normalized {
	case "debug", "info", "warn", "warning", "error":
		if normalized == "warning" {
			return "warn", nil
		}
		return normalized, nil
	case "":
		if mode == "debug" {
			return "debug", nil
		}
		return "info", nil
	default:
		return "", fmt.Errorf("invalid log.level: %q (supported: debug|info|warn|error)", level)
	}
}

func normalizeLogFormat(format, mode string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(format))
	switch normalized {
	case "json", "text":
		return normalized, nil
	case "":
		if mode == "debug" {
			return "text", nil
		}
		return "json", nil
	default:
		return "", fmt.Errorf("invalid log.format: %q (supported: text|json)", format)
	}
}

func normalizeDatabaseLogLevel(level string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(level))
	switch normalized {
	case "silent", "error", "info":
		return normalized, nil
	case "warn", "warning", "":
		return "warn", nil
	default:
		return "", fmt.Errorf("invalid database.log_level: %q (supported: silent|error|warn|info)", level)
	}
}
