package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/syncutil"
)

// RoutineBackupRunner coordinates backup runs for every namespace in a routine.
type RoutineBackupRunner interface {
	// Run resolves target namespaces and starts one cancelable backup per namespace.
	Run(
		ctx context.Context,
		routine *model.BackupRoutine,
		runSpec model.BackupRunSpec,
		logger *slog.Logger,
	) (*BackupNamespacesOperation, error)
}

// RoutineBackupRunnerImpl implements [RoutineBackupRunner] by resolving
// namespaces and delegating each namespace to a [NamespaceBackupRunner].
type RoutineBackupRunnerImpl struct {
	resolver aerospike.NamespaceResolver
	nsRunner NamespaceBackupRunner
}

// NewRoutineBackupRunner returns an [RoutineBackupRunnerImpl] using the
// given per-namespace nsRunner and namespace resolver.
func NewRoutineBackupRunner(
	nsRunner NamespaceBackupRunner,
	namespaceResolver aerospike.NamespaceResolver,
) *RoutineBackupRunnerImpl {
	return &RoutineBackupRunnerImpl{
		resolver: namespaceResolver,
		nsRunner: nsRunner,
	}
}

// Run resolves the namespace list, starts one backup per namespace, and returns the
// aggregate [BackupNamespacesOperation].
func (r *RoutineBackupRunnerImpl) Run(
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
		for h.GetStats() == nil {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
	}

	return &BackupNamespacesOperation{
		handlers: handlers,
	}, nil
}
