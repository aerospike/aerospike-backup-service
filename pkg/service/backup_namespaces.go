package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
)

// BackupNamespacesOperation orchestrates backup operations across multiple namespaces.
// It creates and manages individual BackupOperation instances for each namespace and
// coordinates their execution.
type BackupNamespacesOperation struct {
	namespaces     []string
	routineName    string
	backupService  Backup
	backupPolicy   *model.BackupPolicy
	client         *backup.Client
	retry          executor
	metadataWriter BackupMetadataWriter
	timebounds     model.TimeBounds
	logger         *slog.Logger
	now            time.Time
	isIncremental  bool

	handlers map[string]CancelableBackupHandler
}

// NewBackupNamespacesOperation creates a new BackupNamespacesOperation instance that will
// manage backup operations for multiple namespaces.
func NewBackupNamespacesOperation(
	namespaces []string,
	routineName string,
	backupService Backup,
	backupPolicy *model.BackupPolicy,
	client *backup.Client,
	retry executor,
	metadataWriter BackupMetadataWriter,
	timebounds model.TimeBounds,
	logger *slog.Logger,
	now time.Time,
	isIncremental bool,
) *BackupNamespacesOperation {
	return &BackupNamespacesOperation{
		namespaces:     namespaces,
		routineName:    routineName,
		backupService:  backupService,
		backupPolicy:   backupPolicy,
		client:         client,
		retry:          retry,
		metadataWriter: metadataWriter,
		timebounds:     timebounds,
		logger:         logger,
		now:            now,
		isIncremental:  isIncremental,
		handlers:       make(map[string]CancelableBackupHandler),
	}
}

// Run executes backup operations for all namespaces. It creates and runs individual
// BackupOperation instances for each namespace and waits for their completion.
func (op *BackupNamespacesOperation) Run(ctx context.Context) error {
	for _, namespace := range op.namespaces {
		backupOp := NewBackupOperation(
			namespace,
			op.routineName,
			op.backupService,
			op.backupPolicy,
			op.client,
			op.retry,
			op.metadataWriter,
			op.timebounds,
			op.logger,
			op.now,
			op.isIncremental,
		)

		handler := backupOp.Run(ctx)
		op.handlers[namespace] = handler
	}

	return op.waitForBackups(ctx)
}

// waitForBackups waits for all backup operations to complete and collects any errors
// that occurred during the backup process.
func (op *BackupNamespacesOperation) waitForBackups(ctx context.Context) error {
	var aggregatedErr error
	for ns, handler := range op.handlers {
		if err := handler.Wait(ctx); err != nil {
			aggregatedErr = errors.Join(aggregatedErr, fmt.Errorf("namespace %s: %w", ns, err))
		}
	}
	return aggregatedErr
}

// Cancel stops all running backup operations.
func (op *BackupNamespacesOperation) Cancel() {
	for _, handler := range op.handlers {
		handler.Cancel()
	}
}

// GetHandlers returns the map of namespace to backup handlers.
func (op *BackupNamespacesOperation) GetHandlers() map[string]CancelableBackupHandler {
	return op.handlers
}
