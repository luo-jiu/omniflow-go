package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"omniflow-go/internal/config"
)

type HTTPServer struct {
	server *http.Server
	logger *slog.Logger
}

func NewHTTPServer(cfg *config.Config, handler http.Handler, logger *slog.Logger) *HTTPServer {
	return &HTTPServer{
		logger: logger,
		server: &http.Server{
			Addr:         cfg.Server.Address(),
			Handler:      handler,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		},
	}
}

func (s *HTTPServer) Start() error {
	s.logger.Info("http server listening", "addr", s.server.Addr)

	err := s.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down http server")
	return s.server.Shutdown(ctx)
}
