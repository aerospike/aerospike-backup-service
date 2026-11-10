package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// This file holds the interfaces a backup mode implements. The scan mode implements them in
// pkg/service/scanbackup and the server mode in pkg/service/serverbackup. The rest of this
// package works with a mode only through these interfaces and [StartController].

// BackupReader lists backups by reading the metadata files stored inside backup folders.
type BackupReader interface {
	// GetBackups retrieves backup details based on the provided filter.
	// Returned backups are sorted by Created time in ascending order.
	GetBackups(ctx context.Context, filter BackupFilter) ([]model.BackupDetails, error)
}

// RoutineBackupRunner resolves the namespaces of a routine and starts a backup for each of them.
type RoutineBackupRunner interface {
	// Run resolves target namespaces and starts one cancelable backup per namespace.
	Run(
		ctx context.Context,
		routine *model.BackupRoutine,
		runSpec model.BackupRunSpec,
		logger *slog.Logger,
	) (RoutineBackupHandler, error)
}

// RoutineBackupHandler is the handle of a routine's backup run, which the registry tracks while the
// run lasts and through which it can be canceled. It speaks in the routine's terms only: what a
// backup mode alone knows goes in its slot of the [model.BackupRun] it reports.
type RoutineBackupHandler interface {
	// Wait waits for the run to end and returns the error it ended with, or ctx's error if ctx is
	// done first.
	Wait(ctx context.Context) error
	// Cancel stops the run.
	Cancel()
	// Status is a snapshot of the run, or nil before it has started.
	Status() *model.BackupRun
}

// BackupRetentionManager deletes the backups that fall outside a routine's retention policy.
type BackupRetentionManager interface {
	// ApplyRetention deletes backups that exceed the routine's configured retention limits.
	// It does nothing when the routine has no retention policy, or when the routine's
	// storage is already locked by another delete or restore.
	ApplyRetention(ctx context.Context, routine *model.BackupRoutine) error
}

// ClusterConfigWriter saves the configuration of a routine's source cluster next to the backup
// taken at the given timestamp.
type ClusterConfigWriter interface {
	// Write writes the cluster configuration for the given routine and timestamp.
	Write(ctx context.Context, routine *model.BackupRoutine, timestamp time.Time) error
}

// ConfigRetriever returns the cluster configuration files saved with the last full backup
// at or before a given time, packed into a zip archive.
type ConfigRetriever interface {
	// RetrieveConfiguration returns backed up Aerospike configuration.
	RetrieveConfiguration(context.Context, *model.BackupRoutine, time.Time) ([]byte, error)
}

// RestoreHandler is the handle of a started restore run. It speaks in terms every backup mode
// has: what a mode cannot report stays zero or nil in the [model.RestoreRun] it returns.
type RestoreHandler interface {
	// Wait waits for the run to end and returns the error it ended with.
	Wait(ctx context.Context) error
	// Status is a snapshot of the run.
	Status() model.RestoreRun
}
