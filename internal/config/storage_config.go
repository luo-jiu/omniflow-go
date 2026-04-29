package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// StorageConfig 多 provider 存储配置，从 configs/storage.yaml 独立加载。
type StorageConfig struct {
	Providers       map[string]ProviderConfig `yaml:"providers"`
	DefaultProvider string                    `yaml:"default_provider"`
	RoutingRules    []RoutingRule             `yaml:"routing_rules"`
}

// ProviderConfig 单个存储 provider 的连接配置。
type ProviderConfig struct {
	Type      string `yaml:"type"`
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	UseSSL    bool   `yaml:"use_ssl"`
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`
	Label     string `yaml:"label"`
}

// RoutingRule 文件路由规则，按顺序匹配。
type RoutingRule struct {
	Name           string         `yaml:"name"`
	Conditions     RuleConditions `yaml:"conditions"`
	TargetProvider string         `yaml:"target_provider"`
}

// RuleConditions 路由条件集合，所有非零条件取 AND。
type RuleConditions struct {
	MinFileSizeBytes int64    `yaml:"min_file_size_bytes"`
	MaxFileSizeBytes int64    `yaml:"max_file_size_bytes"`
	Extensions       []string `yaml:"extensions"`
	MIMEPrefixes     []string `yaml:"mime_prefixes"`
}

var (
	ErrStorageConfigInvalid = errors.New("invalid storage config")
)

// LoadStorageConfig 读取并校验 storage.yaml。
func LoadStorageConfig(path string) (*StorageConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read storage config: %w", err)
	}

	var cfg StorageConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal storage config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveStorageConfig 将配置写回文件。
func SaveStorageConfig(path string, cfg *StorageConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal storage config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write storage config: %w", err)
	}
	return nil
}

// Validate 校验配置完整性。
func (c *StorageConfig) Validate() error {
	if len(c.Providers) == 0 {
		return fmt.Errorf("%w: at least one provider is required", ErrStorageConfigInvalid)
	}

	if c.DefaultProvider == "" {
		return fmt.Errorf("%w: default_provider is required", ErrStorageConfigInvalid)
	}
	if _, ok := c.Providers[c.DefaultProvider]; !ok {
		return fmt.Errorf("%w: default_provider %q not found in providers", ErrStorageConfigInvalid, c.DefaultProvider)
	}

	for alias, p := range c.Providers {
		if strings.TrimSpace(alias) == "" {
			return fmt.Errorf("%w: provider alias must not be empty", ErrStorageConfigInvalid)
		}
		if strings.TrimSpace(p.Type) == "" {
			return fmt.Errorf("%w: provider %q type is required", ErrStorageConfigInvalid, alias)
		}
		if strings.TrimSpace(p.Endpoint) == "" {
			return fmt.Errorf("%w: provider %q endpoint is required", ErrStorageConfigInvalid, alias)
		}
		if strings.TrimSpace(p.Bucket) == "" {
			return fmt.Errorf("%w: provider %q bucket is required", ErrStorageConfigInvalid, alias)
		}
	}

	for i, rule := range c.RoutingRules {
		if strings.TrimSpace(rule.TargetProvider) == "" {
			return fmt.Errorf("%w: routing_rules[%d] target_provider is required", ErrStorageConfigInvalid, i)
		}
		if _, ok := c.Providers[rule.TargetProvider]; !ok {
			return fmt.Errorf("%w: routing_rules[%d] target_provider %q not found in providers",
				ErrStorageConfigInvalid, i, rule.TargetProvider)
		}
	}

	return nil
}

// DeriveStorageConfigFromLegacy 将旧版单 provider 配置转换为多 provider 格式。
func DeriveStorageConfigFromLegacy(cfg *Config) *StorageConfig {
	providerType := strings.TrimSpace(strings.ToLower(cfg.Storage.Provider))
	if providerType == "" {
		providerType = "minio"
	}

	alias := providerType

	return &StorageConfig{
		Providers: map[string]ProviderConfig{
			alias: {
				Type:      providerType,
				Endpoint:  cfg.MinIO.Endpoint,
				AccessKey: cfg.MinIO.AccessKey,
				SecretKey: cfg.MinIO.SecretKey,
				UseSSL:    cfg.MinIO.UseSSL,
				Bucket:    cfg.MinIO.Bucket,
				Label:     "MinIO",
			},
		},
		DefaultProvider: alias,
		RoutingRules:    nil,
	}
}

// MaskSecrets 返回配置副本，敏感字段脱敏。
func (c *StorageConfig) MaskSecrets() StorageConfig {
	masked := StorageConfig{
		Providers:       make(map[string]ProviderConfig, len(c.Providers)),
		DefaultProvider: c.DefaultProvider,
		RoutingRules:    c.RoutingRules,
	}
	for alias, p := range c.Providers {
		p.AccessKey = maskString(p.AccessKey)
		p.SecretKey = maskString(p.SecretKey)
		masked.Providers[alias] = p
	}
	return masked
}

func maskString(s string) string {
	if len(s) <= 3 {
		return "***"
	}
	return s[:3] + "***"
}
