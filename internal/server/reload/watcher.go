// Package reload provides a generic mtime-polling file watcher.
package reload

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
)

// Watcher polls a single file path's modification time and invokes onChange when it changes.
type Watcher struct {
	path     string
	interval time.Duration
	onChange func(ctx context.Context) error

	startOnce sync.Once
}

// New returns a Watcher that polls path every interval and calls onChange when mtime changes.
func New(path string, interval time.Duration, onChange func(ctx context.Context) error) *Watcher {
	if interval <= 0 {
		interval = time.Second
	}

	return &Watcher{
		path:     path,
		interval: interval,
		onChange: onChange,
	}
}

// Start begins polling in a background goroutine until ctx is canceled.
// It is safe to call more than once; later calls are ignored.
func (w *Watcher) Start(ctx context.Context) {
	w.startOnce.Do(func() {
		go w.loop(ctx)
	})
}

func (w *Watcher) loop(ctx context.Context) {
	last, err := fileModTime(w.path)
	if err != nil {
		slog.Error("failed to stat watched file", slog.String("path", w.path), attr.Error(err))
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.poll(ctx, &last)
		}
	}
}

func (w *Watcher) poll(ctx context.Context, last *time.Time) {
	modTime, err := fileModTime(w.path)
	if err != nil {
		slog.Error("failed to stat watched file", slog.String("path", w.path), attr.Error(err))
		return
	}

	if modTime.Equal(*last) {
		return
	}

	*last = modTime
	if err := w.onChange(ctx); err != nil {
		slog.Error("file change callback failed", slog.String("path", w.path), attr.Error(err))
	}
}

func fileModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}

	return info.ModTime(), nil
}
