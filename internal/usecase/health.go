package usecase

import (
	"context"
	"log/slog"
	"time"

	"omniflow-go/internal/config"
)

type HealthUseCase struct {
	cfg *config.Config
}

type HealthStatus struct {
	Name      string    `json:"name"`
	Env       string    `json:"env"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

func NewHealthUseCase(cfg *config.Config) *HealthUseCase {
	return &HealthUseCase{cfg: cfg}
}

func (u *HealthUseCase) Status(_ context.Context) HealthStatus {
	status := HealthStatus{
		Name:      u.cfg.App.Name,
		Env:       u.cfg.App.Env,
		Version:   u.cfg.App.Version,
		Timestamp: time.Now().UTC(),
		Status:    "ok",
	}
	slog.Debug("health.status.checked",
		"name", status.Name,
		"env", status.Env,
		"version", status.Version,
		"status", status.Status,
	)
	return status
}
