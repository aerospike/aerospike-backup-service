package service

import (
	"context"
	"log"
	"slices"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/storage"
)

type RetentionManager interface {
	deleteOldBackups(ctx context.Context, namespace string)
}

type RetentionManagerImpl struct {
	backend     BackupListReader
	storage     model.Storage
	routineName string
	policy      *model.RetentionPolicy
}

func NewBackupRetentionManager(
	backend BackupListReader,
	storage model.Storage,
	routineName string,
	policy *model.RetentionPolicy,
) RetentionManager {
	return &RetentionManagerImpl{
		backend:     backend,
		storage:     storage,
		routineName: routineName,
		policy:      policy,
	}
}

func (e *RetentionManagerImpl) deleteOldBackups(ctx context.Context, namespace string) {
	if e.policy == nil || e.policy.FullBackups == nil && e.policy.IncrBackups == nil {
		return // Retention policy is not enabled, do nothing.
	}

	// Fetch full backups once
	fullBackups, err := e.backend.FindFullBackupsForNamespace(ctx, model.TimeBounds{}, namespace)
	if err != nil {
		log.Printf("Error fetching full backups: %v", err)
		return
	}

	slices.SortFunc(fullBackups, func(a, b model.BackupDetails) int {
		return a.Created.Compare(b.Created)
	})

	if e.policy.FullBackups != nil {
		e.deleteExcessFullBackups(ctx, fullBackups, *e.policy.FullBackups)
	}

	if e.policy.IncrBackups != nil {
		e.deleteExcessIncrementalBackups(ctx, fullBackups, *e.policy.IncrBackups, namespace)
	}
}

func (e *RetentionManagerImpl) deleteExcessFullBackups(
	ctx context.Context, fullBackups []model.BackupDetails, retainCount int,
) {
	if len(fullBackups) <= retainCount {
		return // No need to delete any backups.
	}

	// Identify backups to delete
	backupsToDelete := fullBackups[:len(fullBackups)-retainCount]
	e.deleteBackupSlice(ctx, backupsToDelete)
}

func (e *RetentionManagerImpl) deleteExcessIncrementalBackups(
	ctx context.Context, fullBackups []model.BackupDetails, retainCount int, namespace string,
) {
	if len(fullBackups) <= retainCount {
		return // No need to delete incremental backups.
	}

	var earliestToKeep time.Time
	if retainCount == 0 {
		earliestToKeep = time.Now()
	} else {
		earliestToKeep = fullBackups[len(fullBackups)-retainCount].Created
	}

	incrBackups, err := e.backend.FindIncrementalBackupsForNamespace(ctx, model.NewTimeBoundsTo(earliestToKeep), namespace)
	if err != nil {
		log.Printf("Error fetching incremental backups: %v", err)
		return
	}

	// Delete old incremental backups
	e.deleteBackupSlice(ctx, incrBackups)
}

func (e *RetentionManagerImpl) deleteBackupSlice(ctx context.Context, backups []model.BackupDetails) {
	for _, backup := range backups {
		_ = storage.DeleteFolder(ctx, e.storage, backup.Key)
	}
}
