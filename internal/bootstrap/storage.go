package bootstrap

import (
	"log/slog"
	"os"
	"path/filepath"

	"omniflow-go/internal/config"
	objectrepo "omniflow-go/internal/repository/object"
	"omniflow-go/internal/storage"
)

// NewStorageRegistry 创建并初始化多 provider 存储注册表。
// 优先从 configs/storage.yaml 加载；不存在时从 config.yaml 的旧配置派生。
func NewStorageRegistry(cfg *config.Config, logger *slog.Logger) (*storage.StorageRegistry, func(), error) {
	registry := storage.NewStorageRegistry()
	factory := storage.ProviderFactory(objectrepo.NewObjectStorageByConfig)

	storageConfigPath := resolveStorageConfigPath(cfg)

	var storageCfg *config.StorageConfig
	if _, err := os.Stat(storageConfigPath); err == nil {
		loaded, loadErr := config.LoadStorageConfig(storageConfigPath)
		if loadErr != nil {
			return nil, nil, loadErr
		}
		storageCfg = loaded
	} else {
		storageCfg = config.DeriveStorageConfigFromLegacy(cfg)
		logger.Info("storage.yaml not found, using legacy config",
			"default_provider", storageCfg.DefaultProvider,
		)
	}

	if _, err := registry.Reload(storageCfg, factory); err != nil {
		return nil, nil, err
	}

	// 启动热更新 watcher（仅当 storage.yaml 存在时）
	var stopWatcher func()
	if _, err := os.Stat(storageConfigPath); err == nil {
		stop, watchErr := storage.StartConfigWatcher(storageConfigPath, registry, factory, logger)
		if watchErr != nil {
			logger.Warn("storage config watcher start failed, hot-reload disabled", "error", watchErr)
		} else {
			stopWatcher = stop
		}
	}

	cleanup := func() {
		if stopWatcher != nil {
			stopWatcher()
		}
		registry.Close()
	}

	logger.Info("storage registry initialized",
		"providers", registry.Aliases(),
		"default", registry.DefaultAlias(),
	)

	return registry, cleanup, nil
}

func resolveStorageConfigPath(_ *config.Config) string {
	// 默认在同级的 configs/ 目录下查找
	candidates := []string{
		"configs/storage.yaml",
		filepath.Join(filepath.Dir(os.Args[0]), "..", "configs", "storage.yaml"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return "configs/storage.yaml"
}
