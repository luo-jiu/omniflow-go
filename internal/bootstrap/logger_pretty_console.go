package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiGreen  = "\033[32m"
	ansiCyan   = "\033[36m"
)

type prettyConsoleOptions struct {
	Level     slog.Leveler
	AddSource bool
	Color     bool
}

type prettyConsoleHandler struct {
	output io.Writer
	level  slog.Leveler
	opts   prettyConsoleOptions
	attrs  []slog.Attr
	groups []string
	mu     *sync.Mutex
}

func newPrettyConsoleHandler(output io.Writer, opts prettyConsoleOptions) slog.Handler {
	if opts.Level == nil {
		opts.Level = slog.LevelInfo
	}
	return &prettyConsoleHandler{
		output: output,
		level:  opts.Level,
		opts:   opts,
		mu:     &sync.Mutex{},
	}
}

func (h *prettyConsoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *prettyConsoleHandler) Handle(_ context.Context, record slog.Record) error {
	fields := map[string]any{}
	for _, attr := range h.attrs {
		appendAttr(fields, h.groups, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		appendAttr(fields, h.groups, attr)
		return true
	})

	level := strings.ToUpper(record.Level.String())
	if h.opts.Color {
		level = colorizeLevel(record.Level, level)
	}
	isGORMLog := isGORMComponent(fields)
	if isGORMLog {
		stripCommonMeta(fields)
	}

	var builder strings.Builder
	builder.WriteString(record.Time.Format(timeFormat))
	builder.WriteByte('\t')
	builder.WriteString(level)

	if h.opts.AddSource {
		if source := resolveSource(record.PC); source != "" {
			builder.WriteByte('\t')
			builder.WriteString(source)
		}
	}

	builder.WriteByte('\t')
	message := record.Message
	if h.opts.Color && isGORMLog {
		message = colorizeSQLMessage(message)
	}
	builder.WriteString(message)

	if len(fields) > 0 {
		metaRaw, err := json.Marshal(fields)
		if err == nil {
			builder.WriteByte('\t')
			builder.Write(metaRaw)
		}
	}
	builder.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.output, builder.String())
	return err
}

func (h *prettyConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	merged = append(merged, h.attrs...)
	merged = append(merged, attrs...)

	return &prettyConsoleHandler{
		output: h.output,
		level:  h.level,
		opts:   h.opts,
		attrs:  merged,
		groups: append([]string{}, h.groups...),
		mu:     h.mu,
	}
}

func (h *prettyConsoleHandler) WithGroup(name string) slog.Handler {
	if strings.TrimSpace(name) == "" {
		return h
	}
	groups := append([]string{}, h.groups...)
	groups = append(groups, name)

	return &prettyConsoleHandler{
		output: h.output,
		level:  h.level,
		opts:   h.opts,
		attrs:  append([]slog.Attr{}, h.attrs...),
		groups: groups,
		mu:     h.mu,
	}
}

func appendAttr(fields map[string]any, groups []string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return
	}

	keyPath := append([]string{}, groups...)
	if attr.Key != "" {
		keyPath = append(keyPath, attr.Key)
	}

	if attr.Value.Kind() == slog.KindGroup {
		nestedGroups := keyPath
		if len(nestedGroups) == 0 {
			nestedGroups = groups
		}
		for _, nested := range attr.Value.Group() {
			appendAttr(fields, nestedGroups, nested)
		}
		return
	}

	key := strings.Join(keyPath, ".")
	if key == "" {
		key = "_"
	}
	fields[key] = slogValueToAny(attr.Value)
}

func slogValueToAny(value slog.Value) any {
	switch value.Kind() {
	case slog.KindBool:
		return value.Bool()
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindFloat64:
		return value.Float64()
	case slog.KindInt64:
		return value.Int64()
	case slog.KindString:
		return value.String()
	case slog.KindTime:
		return value.Time().Format(timeFormat)
	case slog.KindUint64:
		return value.Uint64()
	case slog.KindLogValuer:
		return slogValueToAny(value.Resolve())
	case slog.KindAny:
		v := value.Any()
		if _, err := json.Marshal(v); err == nil {
			return v
		}
		return fmt.Sprint(v)
	default:
		return fmt.Sprint(value)
	}
}

func resolveSource(pc uintptr) string {
	if pc == 0 {
		return ""
	}
	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	if frame.File == "" {
		return ""
	}

	file := filepath.ToSlash(frame.File)
	segments := strings.Split(file, "/")
	if len(segments) > 3 {
		file = strings.Join(segments[len(segments)-3:], "/")
	}
	return file + ":" + strconv.Itoa(frame.Line)
}

func colorizeLevel(level slog.Level, label string) string {
	switch {
	case level <= slog.LevelDebug:
		return ansiCyan + label + ansiReset
	case level < slog.LevelWarn:
		return ansiGreen + label + ansiReset
	case level < slog.LevelError:
		return ansiYellow + label + ansiReset
	default:
		return ansiRed + label + ansiReset
	}
}

func isGORMComponent(fields map[string]any) bool {
	component, ok := fields["component"]
	if !ok {
		return false
	}
	text, ok := component.(string)
	if !ok {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(text), "gorm")
}

func colorizeSQLMessage(message string) string {
	return ansiGreen + message + ansiReset
}

func stripCommonMeta(fields map[string]any) {
	delete(fields, "component")
	delete(fields, "service")
	delete(fields, "env")
	delete(fields, "version")
}

const timeFormat = "2006-01-02T15:04:05.000-0700"

type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) slog.Handler {
	if len(handlers) == 1 {
		return handlers[0]
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var handleErr error
	for _, handler := range h.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}
		if err := handler.Handle(ctx, record.Clone()); err != nil {
			handleErr = errors.Join(handleErr, err)
		}
	}
	return handleErr
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		next = append(next, handler.WithAttrs(attrs))
	}
	return &multiHandler{handlers: next}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		next = append(next, handler.WithGroup(name))
	}
	return &multiHandler{handlers: next}
}
