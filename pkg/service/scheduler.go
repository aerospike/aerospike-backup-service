package service

import (
	"context"

	"github.com/aerospike/backup/internal/util"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/aerospike/backup/pkg/stdio"
)

// backup service
var backupService shared.Backup = shared.NewBackup()

// stdIO captures standard output
var stdIO *stdio.CgoStdio = &stdio.CgoStdio{}

// ScheduleHandlers schedules the configured backup policies.
func ScheduleHandlers(ctx context.Context, handlers []BackupScheduler) {
	for _, handler := range handlers {
		handler.Schedule(ctx)
	}
}

// BuildBackupHandlers builds a list of BackupSchedulers according to
// the given configuration.
func BuildBackupHandlers(config *model.Config) []BackupScheduler {
	schedulers := make([]BackupScheduler, 0, len(config.BackupPolicies))
	for _, backupRoutine := range config.BackupRoutines {
		handler, err := NewBackupHandler(config, backupRoutine)
		util.Check(err)
		schedulers = append(schedulers, handler)
	}
	return schedulers
}
