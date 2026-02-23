package log

import (
	"context"
	"log/slog"
)

// NewMinLevelLogger returns a logger that forwards records at or above minLevel.
// It reuses the same underlying handler chain as the base logger.
func NewMinLevelLogger(base *slog.Logger, minLevel slog.Level) *slog.Logger {
	return slog.New(&minLevelHandler{
		minLevel: minLevel,
		base:     base.Handler(),
	})
}

type minLevelHandler struct {
	minLevel slog.Level
	base     slog.Handler
}

func (h *minLevelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if level < h.minLevel {
		return false
	}

	return h.base.Enabled(ctx, level)
}

func (h *minLevelHandler) Handle(ctx context.Context, record slog.Record) error {
	return h.base.Handle(ctx, record)
}

func (h *minLevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &minLevelHandler{
		minLevel: h.minLevel,
		base:     h.base.WithAttrs(attrs),
	}
}

func (h *minLevelHandler) WithGroup(name string) slog.Handler {
	return &minLevelHandler{
		minLevel: h.minLevel,
		base:     h.base.WithGroup(name),
	}
}
