package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/syncutil"
)

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

	var handlers = make(map[string]CancelableBackupHandler, len(namespaces))
	for _, namespace := range namespaces {
		handlers[namespace] = r.nsRunner.Run(ctx, routine, namespace, runSpec, scanLimiter, logger)
	}

	for _, h := range handlers {
		if err := waitUntilBackupStarted(ctx, h); err != nil {
			return nil, err
		}
	}

	return &BackupNamespacesOperation{
		handlers: handlers,
	}, nil
}

// waitUntilBackupStarted blocks until the namespace backup pipeline has started.
func waitUntilBackupStarted(ctx context.Context, h CancelableBackupHandler) error {
	for h.GetStats() == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil
}
