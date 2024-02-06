package model

const (
	StateFileName              = "state.yaml"
	IncrementalBackupDirectory = "incremental"
	FullBackupDirectory        = "backup"
	maxRack                    = 1000000 // max possible value https://aerospike.com/docs/server/reference/configuration#namespace__rack-id
)
