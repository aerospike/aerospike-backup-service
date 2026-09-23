package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/try"
	"github.com/aerospike/backup-go/models"
)

// retryableBackupHandler wraps a BackupHandler with retries and cancellation.
type retryableBackupHandler struct {
	sync.RWMutex
	handler backupexecutor.BackupHandler
	cancel  context.CancelFunc
	// started is closed once the pipeline is running. Every retry attempt that gets past Start
	// sets the inner handler again, so the close is guarded.
	started   chan struct{}
	startOnce sync.Once
	// done is closed once the retry loop has returned; err holds its outcome.
	// err is written before done is closed and read only by waiters that have seen it closed,
	// so it needs no lock of its own.
	done chan struct{}
	err  error
}

var _ backupexecutor.BackupHandler = (*retryableBackupHandler)(nil)

// retryableBackupCallbacks configures the backup lifecycle hooks for startRetryableBackup.
type retryableBackupCallbacks struct {
	Start     func(ctx context.Context) (backupexecutor.BackupHandler, error)
	OnSuccess func(ctx context.Context, stats *models.BackupStats) error
	OnRetry   func()
	// AfterSuccess runs once the backup and its OnSuccess have both succeeded. It cannot fail the
	// backup: it is housekeeping on a backup that is already complete.
	AfterSuccess func(ctx context.Context)
}

// startRetryableBackup starts a backup with retries and returns once its pipeline is running.
// The returned handler observes that run: wait for it, read its statistics, cancel it.
//
// A run that gives up before ever starting a pipeline - an unreachable cluster, a storage writer
// that cannot be created - has nothing to observe: it never publishes statistics, so the error
// that ended the run is returned instead of a handler, and nothing is left running.
func startRetryableBackup(
	ctx context.Context,
	policy models.RetryPolicy,
	callbacks retryableBackupCallbacks,
	logger *slog.Logger,
) (*retryableBackupHandler, error) {
	// A run uses two contexts: the caller's, which starts the pipeline and outlives every run, and a
	// cancelable child of it, which is this run's handle and what Cancel ends. Ending the child
	// stops the pipeline, because the inner handler cancels its own work when the context it waits
	// under ends.
	ctxWithCancel, cancel := context.WithCancel(ctx)
	h := &retryableBackupHandler{
		started: make(chan struct{}),
		done:    make(chan struct{}),
		cancel:  cancel,
	}

	// Helper to retry onSuccess only. The loop observes the wait context, so Cancel stops further
	// attempts; the write itself runs on the run context and is left to finish once started.
	retryOnSuccess := func(handler backupexecutor.BackupHandler) error {
		return try.Retry(ctxWithCancel, policy, logger.With(slog.String("label", "write metadata")), func() error {
			return callbacks.OnSuccess(ctx, handler.GetStats())
		}, func() {})
	}

	// Process backup function.
	processBackup := func() error {
		// The pipeline is started on the run context; only the wait is tied to this run's handle.
		handler, err := callbacks.Start(ctx)
		if err != nil {
			return fmt.Errorf("failed to start backup: %w", err)
		}

		h.setHandler(handler)

		if err = handler.Wait(ctxWithCancel); err != nil {
			h.setHandler(nil)
			return fmt.Errorf("backup failed: %w", err)
		}

		if err = retryOnSuccess(handler); err != nil {
			return err
		}

		callbacks.AfterSuccess(ctx)

		return nil
	}

	backupLogger := logger.With(slog.String("label", "backup"))
	run := func() error {
		return try.Retry(ctxWithCancel, policy, backupLogger, processBackup, callbacks.OnRetry)
	}

	if err := h.start(ctx, run); err != nil {
		h.Cancel()

		return nil, err
	}

	return h, nil
}

// start launches the run and blocks until its pipeline is live. Both halves belong together: the
// signal waited for here is produced by the loop started here. The run's wait context is released
// as soon as that loop returns, so a finished run leaves nothing behind in the scheduler context.
func (h *retryableBackupHandler) start(ctx context.Context, run func() error) error {
	go func() {
		defer h.cancel()

		h.err = run()

		close(h.done)
	}()

	return h.waitStarted(ctx)
}

func (h *retryableBackupHandler) setHandler(handler backupexecutor.BackupHandler) {
	h.Lock()
	h.handler = handler
	h.Unlock()

	if handler != nil {
		h.startOnce.Do(func() { close(h.started) })
	}
}

// waitStarted blocks until the backup pipeline is running, and returns the run's own error if
// it ends without ever starting one.
func (h *retryableBackupHandler) waitStarted(ctx context.Context) error {
	select {
	case <-h.started:
		return nil
	case <-h.done:
		// A run can start and finish before this is reached; that is a started run.
		select {
		case <-h.started:
			return nil
		default:
		}

		return h.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *retryableBackupHandler) Wait(ctx context.Context) error {
	select {
	case <-h.done:
		return h.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetStats returns backup statistics from the inner handler, or nil before Start() completes.
func (h *retryableBackupHandler) GetStats() *models.BackupStats {
	h.RLock()
	defer h.RUnlock()
	if h.handler != nil {
		return h.handler.GetStats()
	}

	return nil
}

func (h *retryableBackupHandler) GetMetrics() *models.Metrics {
	h.RLock()
	defer h.RUnlock()
	if h.handler != nil {
		return h.handler.GetMetrics()
	}

	return nil
}

// Cancel ends this run's wait context, which stops the inner handler and the retry loop. The
// same function runs when the loop returns on its own, and a CancelFunc is idempotent, so Cancel
// is safe at any time and any number of times.
func (h *retryableBackupHandler) Cancel() {
	h.cancel()
}
