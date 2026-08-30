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

// fingerprint is mtime plus size. Mtime is the usual signal (atomic rename, cert-manager).
// Size catches a rewrite that keeps the old timestamp (cp -p, second-granularity FS).
type fingerprint struct {
	modTime time.Time
	size    int64
}

func (a fingerprint) equal(b fingerprint) bool {
	return a.modTime.Equal(b.modTime) && a.size == b.size
}

// Watcher polls a file and invokes a callback when the file's mtime or size changes.
type Watcher interface {
	// Start begins polling in a background goroutine until ctx is canceled.
	Start(ctx context.Context)
}

var _ Watcher = (*watcher)(nil)

type watcher struct {
	path     string
	interval time.Duration
	onChange func(ctx context.Context) error
	last     fingerprint

	startOnce sync.Once
}

const defaultInterval = 10 * time.Second

// New returns a Watcher that polls path every interval and calls onChange when the file changes.
// The baseline is taken here, not in Start, so a rewrite between construction and Start is
// still seen on the first tick.
func New(path string, interval time.Duration, onChange func(ctx context.Context) error) Watcher {
	// A non-positive interval panics time.NewTicker.
	if interval <= 0 {
		slog.Warn("invalid file watch interval, using default",
			slog.Duration("requested", interval),
			slog.Duration("default", defaultInterval),
			slog.String("path", path),
		)
		interval = defaultInterval
	}

	last, err := fileFingerprint(path)
	if err != nil {
		slog.Error("failed to stat watched file", slog.String("path", path), attr.Error(err))
	}

	return &watcher{
		path:     path,
		interval: interval,
		onChange: onChange,
		last:     last,
	}
}

// Start begins polling in a background goroutine until ctx is canceled.
// It is safe to call more than once; later calls are ignored.
func (w *watcher) Start(ctx context.Context) {
	w.startOnce.Do(func() {
		go w.loop(ctx)
	})
}

func (w *watcher) loop(ctx context.Context) {
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

func (w *watcher) poll(ctx context.Context) {
	fp, err := fileFingerprint(w.path)
	if err != nil {
		slog.Error("failed to stat watched file", slog.String("path", w.path), attr.Error(err))
		return
	}

	if fp.equal(w.last) {
		return
	}

	if err := w.onChange(ctx); err != nil {
		// Leave last unchanged so the next tick retries.
		slog.Error("file change callback failed", slog.String("path", w.path), attr.Error(err))
		return
	}

	after, err := fileFingerprint(w.path)
	if err != nil {
		slog.Error("failed to stat watched file", slog.String("path", w.path), attr.Error(err))
		return
	}
	if !after.equal(fp) {
		// Rewritten during the callback: keep last so the next tick reloads.
		return
	}

	w.last = after
}

func fileFingerprint(path string) (fingerprint, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fingerprint{}, err
	}

	return fingerprint{modTime: info.ModTime(), size: info.Size()}, nil
}
