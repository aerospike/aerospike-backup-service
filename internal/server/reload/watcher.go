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

type fingerprint struct {
	modTime time.Time
	size    int64
}

func (a fingerprint) equal(b fingerprint) bool {
	return a.modTime.Equal(b.modTime) && a.size == b.size
}

// Watcher polls a single file path's modification time and size and invokes onChange when either changes.
type Watcher struct {
	path     string
	interval time.Duration
	onChange func(ctx context.Context) error
	last     fingerprint

	startOnce sync.Once
}

// New returns a Watcher that polls path every interval and calls onChange when the file changes.
// The baseline fingerprint is taken here so Start cannot miss a rewrite that happened after New.
func New(path string, interval time.Duration, onChange func(ctx context.Context) error) *Watcher {
	if interval <= 0 {
		interval = time.Second
	}

	last, err := fileFingerprint(path)
	if err != nil {
		slog.Error("failed to stat watched file", slog.String("path", path), attr.Error(err))
	}

	return &Watcher{
		path:     path,
		interval: interval,
		onChange: onChange,
		last:     last,
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
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Watcher) poll(ctx context.Context) {
	fp, err := fileFingerprint(w.path)
	if err != nil {
		slog.Error("failed to stat watched file", slog.String("path", w.path), attr.Error(err))
		return
	}

	if fp.equal(w.last) {
		return
	}

	if err := w.onChange(ctx); err != nil {
		slog.Error("file change callback failed", slog.String("path", w.path), attr.Error(err))
		return
	}

	w.last = fp
}

func fileFingerprint(path string) (fingerprint, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fingerprint{}, err
	}

	return fingerprint{modTime: info.ModTime(), size: info.Size()}, nil
}
