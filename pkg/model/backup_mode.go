package model

// BackupMode determines how backup data is read from the cluster.
type BackupMode string

const (
	BackupModeScan   BackupMode = "scan"
	BackupModeServer BackupMode = "server"
)
