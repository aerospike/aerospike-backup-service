package util

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/reugn/go-quartz/logger"
	"github.com/reugn/go-quartz/quartz"
)

func init() {
	quartz.Sep = "_"
}

// QuartzLogger implements the quartz logger interface.
type QuartzLogger struct {
	ctx context.Context
}

var _ logger.Logger = (*QuartzLogger)(nil)

// NewQuartzLogger returns a new QuartzLogger.
func NewQuartzLogger(ctx context.Context) logger.Logger {
	return &QuartzLogger{
		ctx: ctx,
	}
}

// Trace logs at LevelTrace.
func (l *QuartzLogger) Trace(msg any) {
	slog.Log(l.ctx, slog.LevelDebug, fmt.Sprint(msg))
}

// Tracef logs at LevelTrace.
func (l *QuartzLogger) Tracef(format string, args ...any) {
	slog.Log(l.ctx, slog.LevelDebug, fmt.Sprintf(format, args...))
}

// Debug logs at LevelDebug.
func (l *QuartzLogger) Debug(msg any) {
	slog.Debug(fmt.Sprint(msg))
}

// Debugf logs at LevelDebug.
func (l *QuartzLogger) Debugf(format string, args ...any) {
	slog.Debug(fmt.Sprintf(format, args...))
}

// Info logs at LevelInfo.
func (l *QuartzLogger) Info(msg any) {
	slog.Info(fmt.Sprint(msg))
}

// Infof logs at LevelInfo.
func (l *QuartzLogger) Infof(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...))
}

// Warn logs at LevelWarn.
func (l *QuartzLogger) Warn(msg any) {
	slog.Warn(fmt.Sprint(msg))
}

// Warnf logs at LevelWarn.
func (l *QuartzLogger) Warnf(format string, args ...any) {
	slog.Warn(fmt.Sprintf(format, args...))
}

// Error logs at LevelError.
func (l *QuartzLogger) Error(msg any) {
	slog.Error(fmt.Sprint(msg))
}

// Errorf logs at LevelError.
func (l *QuartzLogger) Errorf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
}

// Enabled for all.
func (l *QuartzLogger) Enabled(_ logger.Level) bool {
	return true
}
