package model

// BackupMode selects how the whole service instance backs up and restores data.
// An empty mode is the default, scan.
type BackupMode string

const (
	// BackupModeScan is the default mode: the service scans the cluster and writes the data itself.
	BackupModeScan BackupMode = "scan"
	// BackupModeServer delegates backup and restore to the Aerospike server, which writes to S3 directly.
	BackupModeServer BackupMode = "server"
)

// MaxServerRestoreChain is the most backups a server restore job applies: a full backup and the
// incremental backups after it (AS_RESTORE_CHAIN_MAX_LINKS).
const MaxServerRestoreChain = 32
