package service

import "github.com/aerospike/backup/pkg/model"

// BackupListReader allows to read list of existing backups
type BackupListReader interface {
	// FullBackupList returns a list of available full backups.
	// The parameters are timestamp filters by creation time (epoch millis),
	// where from is inclusive and to is exclusive.
	FullBackupList(timebounds *model.TimeBounds) ([]model.BackupDetails, error)

	// IncrementalBackupList returns a list of available incremental backups.
	// The parameters are timestamp filters by creation time (epoch millis),
	// where from is inclusive and to is exclusive.
	IncrementalBackupList(timebounds *model.TimeBounds) ([]model.BackupDetails, error)
}

// BackendsToReaders converts a map of *BackupBackend to a map of BackupListReader.
func BackendsToReaders(backends map[string]*BackupBackend) map[string]BackupListReader {
	result := make(map[string]BackupListReader, len(backends))
	for key, value := range backends {
		result[key] = value
	}
	return result
}
