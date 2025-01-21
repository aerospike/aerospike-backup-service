package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

type Starter struct {
	starter *BackupStarter
}

func NewStarter(routineName string,
	backupService Backup,
	backupPolicy *model.BackupPolicy,
	retry executor,
	metadataWriter BackupMetadataWriter,
	isIncremental bool,
	logger *slog.Logger,
) *Starter {
	return &Starter{
		starter: NewBackupStarter(routineName, backupService, backupPolicy, retry, metadataWriter, isIncremental, logger),
	}
}

func (s *Starter) Start(
	ctx context.Context,
	client *backup.Client,
	namespaces []string,
	timebounds model.TimeBounds,
	now time.Time,
) *BackupNamespacesOperation {
	op := &BackupNamespacesOperation{
		handlers: make(map[string]CancelableBackupHandler, len(namespaces)),
	}

	for _, namespace := range namespaces {
		op.handlers[namespace] = s.starter.Run(ctx, client, namespace, now, timebounds)
	}

	return op
}

// BackupNamespacesOperation orchestrates backup operations across multiple namespaces.
// It creates and manages individual BackupOperation instances for each namespace and
// coordinates their execution.
type BackupNamespacesOperation struct {
	handlers map[string]CancelableBackupHandler
}

var _ CancelableBackupHandler = (*BackupNamespacesOperation)(nil)

// Wait waits for all backup operations to complete and collects any errors
// that occurred during the backup process.
func (op *BackupNamespacesOperation) Wait(ctx context.Context) error {
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

func (op *BackupNamespacesOperation) GetStats() *models.BackupStats {
	activeHandlers := 0

	res := &models.BackupStats{}
	for _, handler := range op.handlers {
		if handler.GetStats() == nil {
			continue
		}

		activeHandlers++
		res.TotalRecords += handler.GetStats().TotalRecords
		res.ReadRecords.Add(handler.GetStats().GetReadRecords())
		res.BytesWritten.Add(handler.GetStats().BytesWritten.Load())

		// These are the backups of multiple namespaces in the same routine.
		// Therefore, picking any of those is valid, since they started at
		// the same time.
		res.StartTime = handler.GetStats().StartTime
	}

	if activeHandlers == 0 {
		return nil
	}

	return res
}
