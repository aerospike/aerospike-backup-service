package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/syncutil"
	"golang.org/x/sync/errgroup"
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

	handlers, err := r.startNamespaces(ctx, routine, runSpec, namespaces, scanLimiter, logger)
	if err != nil {
		return nil, err
	}

	return &BackupNamespacesOperation{
		handlers: handlers,
	}, nil
}

// startNamespaces starts every namespace backup of the run and returns once they are all
// running. If any of them cannot be started, the ones that did start are canceled: they share
// the failed run's timestamp folder and have nobody left to wait for them.
func (r *routineBackupRunner) startNamespaces(
	ctx context.Context,
	routine *model.BackupRoutine,
	runSpec model.BackupRunSpec,
	namespaces []string,
	scanLimiter syncutil.Limiter,
	logger *slog.Logger,
) (map[string]CancelableBackupHandler, error) {
	var (
		mu       sync.Mutex
		handlers = make(map[string]CancelableBackupHandler, len(namespaces))
		group    errgroup.Group
	)

	for _, namespace := range namespaces {
		group.Go(func() error {
			h, err := r.nsRunner.Run(ctx, routine, namespace, runSpec, scanLimiter, logger)
			if err != nil {
				return fmt.Errorf("namespace %s: %w", namespace, err)
			}

			mu.Lock()
			defer mu.Unlock()
			handlers[namespace] = h

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		for _, h := range handlers {
			h.Cancel()
		}

		return nil, err
	}

	return handlers, nil
}
