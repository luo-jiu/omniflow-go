package audit

import (
	"context"
	"log/slog"
	"time"

	"omniflow-go/internal/actor"
)

type Event struct {
	Actor      actor.Actor
	Action     string
	Resource   string
	Success    bool
	OccurredAt time.Time
	Metadata   map[string]any
}

type Sink interface {
	Write(ctx context.Context, event Event) error
}

type LogSink struct {
	logger *slog.Logger
}

func NewLogSink(logger *slog.Logger) *LogSink {
	return &LogSink{logger: logger}
}

func (s *LogSink) Write(_ context.Context, event Event) error {
	s.logger.Info("audit event",
		"actor_id", event.Actor.ID,
		"actor_kind", event.Actor.Kind,
		"action", event.Action,
		"resource", event.Resource,
		"success", event.Success,
		"occurred_at", event.OccurredAt,
		"metadata", event.Metadata,
	)
	return nil
}
