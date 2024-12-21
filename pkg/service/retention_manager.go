package service

import (
	"context"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/storage"
	"log"
	"slices"
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
	if e.policy == nil || ((*e.policy).FullBackups == nil && (*e.policy).IncrBackups == nil) {
		return // Retention policy is not enabled, do nothing.
	}

	// Fetch full backups once
	fullBackups, err := e.backend.FullBackupList(ctx, model.TimeBounds{})
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
		e.deleteExcessIncrementalBackups(ctx, fullBackups, *e.policy.IncrBackups)
	}
}

func (e *RetentionManagerImpl) deleteExcessFullBackups(ctx context.Context, fullBackups []model.BackupDetails, retainCount int) {
	if len(fullBackups) <= retainCount {
		return // No need to delete any backups.
	}

	// Identify backups to delete
	backupsToDelete := fullBackups[:len(fullBackups)-retainCount]
	e.deleteBackupSlice(ctx, backupsToDelete)
}

func (e *RetentionManagerImpl) deleteExcessIncrementalBackups(ctx context.Context, fullBackups []model.BackupDetails, retainCount int) {
	if len(fullBackups) <= retainCount {
		return // No need to delete incremental backups.
	}

	fullBackupsToKeep := fullBackups[len(fullBackups)-retainCount:]

	earliestToKeep := fullBackupsToKeep[0].Created
	incrBackups, err := e.backend.IncrementalBackupList(ctx, model.TimeBounds{
		ToTime: &earliestToKeep,
	})
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
