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

// BackupOperation represents a backup operation for a single namespace. It encapsulates
// all the logic needed to perform a backup of one namespace, including running the backup,
// managing metadata, and handling cleanup.
type BackupOperation struct {
	namespace      string
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
}

// NewBackupOperation creates a new BackupOperation instance with all necessary dependencies
// for performing a single namespace backup.
func NewBackupOperation(
	namespace string,
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
) *BackupOperation {
	return &BackupOperation{
		namespace:      namespace,
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
	}
}

// Run executes the backup operation for the namespace. It handles the entire backup process
// including folder management, metadata writing, and error handling.
func (op *BackupOperation) Run(ctx context.Context) CancelableBackupHandler {
	backupFolder := op.getBackupPath()

	return startBackup(
		ctx,
		op.retry,
		func(ctx context.Context) (BackupHandler, error) {
			return op.backupService.BackupRun(ctx, op.client, op.backupPolicy, op.timebounds, op.namespace, backupFolder)
		},
		func(ctx context.Context) {
			op.deleteFolder(ctx, backupFolder)
		},
		func(ctx context.Context, stats *models.BackupStats) error {
			// For incremental backups, skip metadata for empty backups
			if op.isIncremental && stats.IsEmpty() {
				return nil
			}
			metadata := model.NewMetadataFromStats(stats, op.namespace, util.ValueOrZero(op.timebounds.FromTime), op.now)
			return op.writeBackupMetadata(ctx, metadata, backupFolder)
		},
	)
}

func (op *BackupOperation) getBackupPath() string {
	if op.isIncremental {
		return getIncrementalPath(op.routineName, op.namespace, op.now)
	}
	return getFullPath(op.routineName, op.namespace, op.now)
}

func (op *BackupOperation) deleteFolder(ctx context.Context, path string) {
	err := op.metadataWriter.deleteFolder(ctx, path)
	if err != nil {
		op.logger.Error("Could not delete folder", slog.Any("err", err))
	}
	op.logger.Debug("Deleted folder", slog.String("path", path))
}

func (op *BackupOperation) writeBackupMetadata(
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
