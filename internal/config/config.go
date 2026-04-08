package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App      App      `yaml:"app"`
	Server   Server   `yaml:"server"`
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

type Database struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	LogLevel        string        `yaml:"log_level"`
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
		cfg.applyDefaults()
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg.applyDefaults()
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg.applyDefaults()
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
			Port:            8848,
			Mode:            "debug",
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    15 * time.Second,
			IdleTimeout:     60 * time.Second,
			ShutdownTimeout: 10 * time.Second,
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
			Endpoint:  "127.0.0.1:9000",
			AccessKey: "admin",
			SecretKey: "admin123",
			UseSSL:    false,
			Bucket:    "my-bucket",
		},
	}
}

func (c *Config) applyDefaults() {
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
		c.Server.Port = 8848
	}
	if c.Server.Mode == "" {
		c.Server.Mode = "debug"
	}
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

	if c.Database.MaxOpenConns == 0 {
		c.Database.MaxOpenConns = 20
	}
	if c.Database.MaxIdleConns == 0 {
		c.Database.MaxIdleConns = 10
	}
	if c.Database.ConnMaxLifetime == 0 {
		c.Database.ConnMaxLifetime = 30 * time.Minute
	}
	if c.Database.LogLevel == "" {
		c.Database.LogLevel = "warn"
	}

	if c.Storage.Provider == "" {
		c.Storage.Provider = "minio"
	}

	if c.MinIO.Bucket == "" {
		c.MinIO.Bucket = "my-bucket"
	}
}
