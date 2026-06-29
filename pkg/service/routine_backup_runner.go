package service

import (
	"context"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"golang.org/x/sync/semaphore"
)

// RoutineBackupRunner coordinates backup runs for every namespace in a routine.
type RoutineBackupRunner interface {
	// Run resolves target namespaces and starts one cancelable backup per namespace.
	Run(
		ctx context.Context,
		routine *model.BackupRoutine,
		runSpec model.BackupRunSpec,
		logger *slog.Logger,
	) (CancelableBackupHandler, error)
}

// RoutineBackupRunnerImpl implements [RoutineBackupRunner] by resolving
// namespaces and delegating each namespace to a [NamespaceBackupRunner].
type RoutineBackupRunnerImpl struct {
	resolver aerospike.NamespaceResolver
	nsRunner NamespaceBackupRunner
}

// NewRoutineBackupRunner returns a [RoutineBackupRunnerImpl] using the
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
) (CancelableBackupHandler, error) {
	namespaces, err := r.resolver.ResolveNamespaces(ctx, routine, logger)
	if err != nil {
		return nil, err
	}

	// Create a per-routine semaphore to limit concurrent scans across all namespaces.
	// This ensures fair resource allocation when routines with multiple namespaces
	// compete with single-namespace routines for the global cluster scan limit.
	scanLimiter := semaphore.NewWeighted(int64(routine.BackupPolicy.GetParallelOrDefault()))

	op := &BackupNamespacesOperation{
		handlers: make(map[string]CancelableBackupHandler, len(namespaces)),
	}

	for _, namespace := range namespaces {
		op.handlers[namespace] = r.nsRunner.Run(ctx, routine, namespace, runSpec, scanLimiter, logger)
	}

	return op, nil
}
