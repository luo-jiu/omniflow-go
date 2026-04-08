package app

import (
	"context"
	"fmt"
	"log/slog"

	"omniflow-go/internal/config"
	"omniflow-go/internal/server"
)

type App struct {
	cfg        *config.Config
	logger     *slog.Logger
	httpServer *server.HTTPServer
}

func New(cfg *config.Config, logger *slog.Logger, httpServer *server.HTTPServer) *App {
	return &App{
		cfg:        cfg,
		logger:     logger,
		httpServer: httpServer,
	}
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		errCh <- a.httpServer.Start()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		a.logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	if err := <-errCh; err != nil {
		return err
	}

	a.logger.Info("application stopped cleanly")
	return nil
}
