package service

import (
	"context"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
)

// AllNamespacesBackupRunner coordinates backup runs for every namespace in a routine
// (explicitly configured or discovered on the source cluster).
type AllNamespacesBackupRunner interface {
	// StartBackup resolves target namespaces and starts one cancelable backup per namespace.
	StartBackup(
		ctx context.Context,
		logger *slog.Logger,
		backupRoutine *model.BackupRoutine,
		runSpec model.BackupRunSpec,
	) (*BackupNamespacesOperation, error)
}

// AllNamespacesBackupRunnerImpl implements [AllNamespacesBackupRunner] by resolving
// namespaces and delegating each namespace to a [SingleNamespaceExecutor].
type AllNamespacesBackupRunnerImpl struct {
	resolver aerospike.NamespaceResolver
	executor SingleNamespaceExecutor
}

// NewAllNamespacesBackupRunner returns an [AllNamespacesBackupRunnerImpl] using the
// given per-namespace executor and namespace resolver.
func NewAllNamespacesBackupRunner(
	executor SingleNamespaceExecutor,
	namespaceResolver aerospike.NamespaceResolver,
) *AllNamespacesBackupRunnerImpl {
	return &AllNamespacesBackupRunnerImpl{
		resolver: namespaceResolver,
		executor: executor,
	}
}

// StartBackup resolves the namespace list (configured or discovered from the source
// cluster), starts one [CancelableBackupHandler] per namespace, and returns the
// aggregate [BackupNamespacesOperation].
func (r *AllNamespacesBackupRunnerImpl) StartBackup(
	ctx context.Context,
	logger *slog.Logger,
	backupRoutine *model.BackupRoutine,
	runSpec model.BackupRunSpec,
) (*BackupNamespacesOperation, error) {
	namespaces, err := r.resolver.ResolveNamespaces(ctx, backupRoutine, logger)
	if err != nil {
		return nil, err
	}

	op := &BackupNamespacesOperation{
		handlers: make(map[string]CancelableBackupHandler, len(namespaces)),
	}

	for _, namespace := range namespaces {
		op.handlers[namespace] = r.executor.Run(ctx, logger, backupRoutine, namespace, runSpec)
	}

	return op, nil
}
