package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/syncutil"
)

// startPollInterval is how often Run checks whether a namespace backup has published its
// statistics. A run that ends before it does wakes the wait immediately through Done.
const startPollInterval = 100 * time.Millisecond

// RoutineBackupRunner resolves the namespaces of a routine and starts a backup for each of them.
type RoutineBackupRunner interface {
	// Run resolves target namespaces and starts one cancelable backup per namespace.
	Run(
		ctx context.Context,
		routine *model.BackupRoutine,
		runSpec model.BackupRunSpec,
		logger *slog.Logger,
	) (*BackupNamespacesOperation, error)
}

// routineBackupRunner resolves the namespaces and delegates each one to a [NamespaceBackupRunner].
type routineBackupRunner struct {
	resolver aerospike.NamespaceResolver
	nsRunner NamespaceBackupRunner
}

var _ RoutineBackupRunner = (*routineBackupRunner)(nil)

// NewRoutineBackupRunner returns a RoutineBackupRunner.
func NewRoutineBackupRunner(
	nsRunner NamespaceBackupRunner,
	namespaceResolver aerospike.NamespaceResolver,
) RoutineBackupRunner {
	return &routineBackupRunner{
		resolver: namespaceResolver,
		nsRunner: nsRunner,
	}
}

// Run resolves the namespace list, starts one backup per namespace, and returns the
// aggregate [BackupNamespacesOperation].
func (r *routineBackupRunner) Run(
	ctx context.Context,
	routine *model.BackupRoutine,
	runSpec model.BackupRunSpec,
	logger *slog.Logger,
) (*BackupNamespacesOperation, error) {
	namespaces, err := r.resolver.ResolveNamespaces(ctx, routine, logger)
	if err != nil {
		return nil, err
	}

	// Create the per-routine semaphore to limit concurrent scans.
	// This ensures fair resource allocation between namespaces.
	routineParallelism := int64(routine.BackupPolicy.GetParallelOrDefault())
	scanLimiter := syncutil.NewRandomSemaphore(routineParallelism)
	if err = scanLimiter.Acquire(ctx, routineParallelism); err != nil {
		return nil, err
	}
	defer scanLimiter.Release(routineParallelism)

	var handlers = make(map[string]NamespaceBackupHandler, len(namespaces))
	for _, namespace := range namespaces {
		handlers[namespace] = r.nsRunner.Run(ctx, routine, namespace, runSpec, scanLimiter, logger)
	}

	for namespace, h := range handlers {
		if err := waitUntilBackupStarted(ctx, h); err != nil {
			// The namespaces that did start have nobody left to wait for them, and they share
			// this run's timestamp folder with the one that failed.
			cancelAll(handlers)

			return nil, fmt.Errorf("namespace %s: %w", namespace, err)
		}
	}

	return &BackupNamespacesOperation{
		handlers: handlers,
	}, nil
}

// waitUntilBackupStarted blocks until the namespace backup pipeline has started, or until the
// run ends without one. A Start that keeps failing leaves the statistics nil for as long as the
// run's context lives, which for a scheduled backup is until the service shuts down.
func waitUntilBackupStarted(ctx context.Context, h NamespaceBackupHandler) error {
	for h.GetStats() == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-h.Done():
			// The run is over. Either it never started, and Err says why, or it finished
			// between two polls and there is nothing left to wait for.
			return h.Err()
		case <-time.After(startPollInterval):
		}
	}

	return nil
}

// cancelAll stops every namespace backup of a run that will not be reported on.
func cancelAll(handlers map[string]NamespaceBackupHandler) {
	for _, h := range handlers {
		h.Cancel()
	}
}
