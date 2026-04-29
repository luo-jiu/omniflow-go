package storage

import (
	"log/slog"
	"sync"
	"time"

	"omniflow-go/internal/config"

	"github.com/fsnotify/fsnotify"
)

// StartConfigWatcher 监听 storage.yaml 变更，触发 registry 热加载。
// 返回 stop 函数用于关闭 watcher。
func StartConfigWatcher(
	path string,
	registry *StorageRegistry,
	factory ProviderFactory,
	logger *slog.Logger,
) (stop func(), err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(path); err != nil {
		watcher.Close()
		return nil, err
	}

	stopCh := make(chan struct{})
	var once sync.Once

	go watchLoop(watcher, path, registry, factory, logger, stopCh)

	return func() {
		once.Do(func() {
			close(stopCh)
			watcher.Close()
		})
	}, nil
}

func watchLoop(
	watcher *fsnotify.Watcher,
	path string,
	registry *StorageRegistry,
	factory ProviderFactory,
	logger *slog.Logger,
	stopCh <-chan struct{},
) {
	var debounceTimer *time.Timer
	const debounceDelay = 500 * time.Millisecond

	for {
		select {
		case <-stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}

			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDelay, func() {
				reloadFromFile(path, registry, factory, logger)
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logger.Error("storage config watcher error", "error", err)
		}
	}
}

func reloadFromFile(
	path string,
	registry *StorageRegistry,
	factory ProviderFactory,
	logger *slog.Logger,
) {
	cfg, err := config.LoadStorageConfig(path)
	if err != nil {
		logger.Error("storage config reload: parse failed, keeping current config", "error", err)
		return
	}

	removed, err := registry.Reload(cfg, factory)
	if err != nil {
		logger.Error("storage config reload: apply failed, keeping current config", "error", err)
		return
	}

	logger.Info("storage config reloaded successfully",
		"providers", registry.Aliases(),
		"default", registry.DefaultAlias(),
		"removed", removed,
	)
}
