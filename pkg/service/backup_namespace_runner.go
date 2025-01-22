package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// BackupNamespaceRunner starts a backup operation for a single namespace. It encapsulates
// all the logic needed to perform a backup of one namespace, including running the backup,
// managing metadata, and handling cleanup.
type BackupNamespaceRunner struct {
	routineName    string
	backupService  Backup
	backupPolicy   *model.BackupPolicy
	retry          executor
	metadataWriter BackupMetadataWriter
	logger         *slog.Logger
	backupType     jobType
}

// NewBackupNamespaceRunner creates a new BackupNamespaceRunner instance with all necessary dependencies
// for performing a single namespace backup.
func NewBackupNamespaceRunner(
	routineName string,
	backupService Backup,
	backupPolicy *model.BackupPolicy,
	retry executor,
	metadataWriter BackupMetadataWriter,
	backupType jobType,
	logger *slog.Logger,
) *BackupNamespaceRunner {
	return &BackupNamespaceRunner{
		routineName:    routineName,
		backupService:  backupService,
		backupPolicy:   backupPolicy,
		retry:          retry,
		metadataWriter: metadataWriter,
		logger:         logger,
		backupType:     backupType,
	}
}

// Run executes the backup operation for the namespace. It handles the entire backup process
// including folder management, metadata writing, and error handling.
func (op *BackupNamespaceRunner) Run(
	ctx context.Context,
	client *backup.Client,
	namespace string,
	now time.Time,
	timeBounds model.TimeBounds,
) CancelableBackupHandler {
	backupFolder := getBackupPath(op.routineName, op.backupType, namespace, now)

	return startBackup(
		ctx,
		op.retry,
		func(ctx context.Context) (BackupHandler, error) {
			return op.backupService.BackupRun(ctx, client, op.backupPolicy, timeBounds, namespace, backupFolder)
		},
		func(ctx context.Context) {
			op.deleteFolder(ctx, backupFolder)
		},
		func(ctx context.Context, stats *models.BackupStats) error {
			// For incremental backups, skip metadata for empty backups
			if op.backupType == jobTypeIncremental && stats.IsEmpty() {
				return nil
			}
			metadata := model.NewMetadataFromStats(stats, namespace, util.ValueOrZero(timeBounds.FromTime), now)
			return op.writeBackupMetadata(ctx, metadata, backupFolder)
		},
	)
}

func (op *BackupNamespaceRunner) deleteFolder(ctx context.Context, path string) {
	err := op.metadataWriter.deleteFolder(ctx, path)
	if err != nil {
		op.logger.Error("Could not delete folder", slog.Any("err", err))
		return
	}
	op.logger.Debug("Deleted folder", slog.String("path", path))
}

func (op *BackupNamespaceRunner) writeBackupMetadata(
	ctx context.Context, metadata model.BackupMetadata, backupFolder string,
) error {
	if err := op.metadataWriter.writeBackupMetadata(ctx, backupFolder, metadata); err != nil {
		op.logger.Error("Could not Write backup metadata",
			slog.String("folder", backupFolder),
			slog.Any("err", err))

		return fmt.Errorf("could not write backup metadata to %q: %w", backupFolder, err)
	}

	op.logger.Info("Write backup metadata",
		slog.Any("folder", backupFolder),
		slog.Any("metadata", metadata))

	return nil
}
