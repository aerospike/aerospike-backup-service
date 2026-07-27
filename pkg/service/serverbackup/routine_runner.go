package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go/models"
)

// RoutineRunner runs server-side backups sequentially, one namespace at a time.
type RoutineRunner struct {
	resolver aerospike.NamespaceResolver
	nsRunner *namespaceRunner
}

// NewRoutineRunner returns a runner that executes server backups namespace-by-namespace.
func NewRoutineRunner(
	nsRunner *namespaceRunner,
	namespaceResolver aerospike.NamespaceResolver,
) *RoutineRunner {
	return &RoutineRunner{
		resolver: namespaceResolver,
		nsRunner: nsRunner,
	}
}

// Run resolves namespaces and returns a handler that starts each namespace backup sequentially in Wait.
func (r *RoutineRunner) Run(
	ctx context.Context,
	routine *model.BackupRoutine,
	runSpec model.BackupRunSpec,
	logger *slog.Logger,
) (service.CancelableBackupHandler, error) {
	namespaces, err := r.resolver.ResolveNamespaces(ctx, routine, logger)
	if err != nil {
		return nil, err
	}

	return &sequentialOperation{
		routine:    routine,
		runSpec:    runSpec,
		namespaces: namespaces,
		nsRunner:   r.nsRunner,
		logger:     logger,
	}, nil
}

type sequentialOperation struct {
	mu sync.RWMutex

	current service.CancelableBackupHandler

	routine    *model.BackupRoutine
	runSpec    model.BackupRunSpec
	namespaces []string
	nsRunner   *namespaceRunner
	logger     *slog.Logger
}

var _ service.CancelableBackupHandler = (*sequentialOperation)(nil)

func (op *sequentialOperation) Wait(ctx context.Context) error {
	var aggregatedErr error

	for _, namespace := range op.namespaces {
		handler := op.nsRunner.Run(ctx, op.routine, namespace, op.runSpec, op.logger)
		op.setCurrent(handler)

		if err := handler.Wait(ctx); err != nil {
			aggregatedErr = errors.Join(aggregatedErr, fmt.Errorf("namespace %s: %w", namespace, err))
		}
	}

	op.setCurrent(nil)

	return aggregatedErr
}

func (op *sequentialOperation) Cancel() {
	op.mu.RLock()
	current := op.current
	op.mu.RUnlock()

	if current != nil {
		current.Cancel()
	}
}

func (op *sequentialOperation) GetMetrics() *models.Metrics {
	op.mu.RLock()
	current := op.current
	op.mu.RUnlock()

	if current != nil {
		return current.GetMetrics()
	}

	return nil
}

func (op *sequentialOperation) GetStats() *models.BackupStats {
	op.mu.RLock()
	current := op.current
	op.mu.RUnlock()

	if current != nil {
		return current.GetStats()
	}

	return nil
}

func (op *sequentialOperation) setCurrent(handler service.CancelableBackupHandler) {
	op.mu.Lock()
	defer op.mu.Unlock()
	op.current = handler
}
