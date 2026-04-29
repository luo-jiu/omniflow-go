package repository

import (
	"fmt"
	"strings"

	"omniflow-go/internal/config"
	objectminio "omniflow-go/internal/repository/object/minio"
	"omniflow-go/internal/storage"
)

const providerMinIO = "minio"

// NewObjectStorage 根据配置选择对象存储实现（旧版单 provider 入口）。
func NewObjectStorage(cfg *config.Config) (storage.ObjectStorage, func(), error) {
	provider := strings.TrimSpace(strings.ToLower(cfg.Storage.Provider))
	if provider == "" {
		provider = providerMinIO
	}

	switch provider {
	case providerMinIO:
		return objectminio.NewStore(cfg)
	case "s3", "oss", "cos", "obs":
		return nil, func() {}, fmt.Errorf("%w: %s", storage.ErrProviderNotImplemented, provider)
	default:
		return nil, func() {}, fmt.Errorf("%w: %s", storage.ErrProviderUnknown, provider)
	}
}

// NewObjectStorageByConfig 根据 ProviderConfig 创建存储实例（多 provider 入口）。
func NewObjectStorageByConfig(alias string, cfg config.ProviderConfig) (storage.ObjectStorage, func(), error) {
	providerType := strings.TrimSpace(strings.ToLower(cfg.Type))
	if providerType == "" {
		return nil, nil, fmt.Errorf("%w: empty provider type for %q", storage.ErrProviderUnknown, alias)
	}

	switch providerType {
	case providerMinIO:
		return objectminio.NewStoreFromConfig(alias, cfg)
	case "s3", "oss", "cos", "obs":
		return nil, nil, fmt.Errorf("%w: %s", storage.ErrProviderNotImplemented, providerType)
	default:
		return nil, nil, fmt.Errorf("%w: %s", storage.ErrProviderUnknown, providerType)
	}
}
