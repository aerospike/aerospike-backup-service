package model

import (
	"bytes"
	"encoding/gob"
)

// BackupRoutine represents a scheduled backup operation routine.
type BackupRoutine struct {
	// The unique name of the routine (key in routines map).
	Name string

	// The corresponding backup policy.
	BackupPolicy *BackupPolicy
	// The corresponding source cluster.
	SourceCluster *AerospikeCluster
	// The corresponding storage provider configuration.
	Storage Storage
	// The Secret Agent configuration for the routine (optional).
	SecretAgent *SecretAgent
	// The interval for full backup as a cron expression string.
	IntervalCron string
	// The interval for incremental backup as a cron expression string (optional).
	IncrIntervalCron string
	// The list of the namespaces to back up (optional, empty list implies backup up whole cluster).
	Namespaces []string
	// The list of backup set names (optional, an empty list implies backing up all sets).
	SetList []string
	// The list of backup bin names (optional, an empty list implies backing up all bins).
	BinList []string
	// A list of Aerospike Server rack IDs to use when reading records for a backup.
	RackList []int
	// Back up list of partition filters. Partition filters can be ranges or individual partitions.
	// Default number of partitions to back up: 0 to 4095: all partitions.
	PartitionList string
	// NodeList contains a list of nodes to back up.
	NodeList []string
	// Whether this routine is disabled and should not run.
	Disabled bool
}

func init() {
	// Register all concrete types that can appear in interface fields of BackupRoutine
	// with encoding/gob. BackupRoutine.Copy() uses gob to perform a deep copy via
	// encode/decode round-trip. If a concrete implementation of an interface
	// is not registered here, gob will panic at runtime when encoding/decoding that value.
	gob.Register(&LocalStorage{})
	gob.Register(&S3Storage{})
	gob.Register(&GcpStorage{})
	gob.Register(&AzureStorage{})
	gob.Register(&AzureADAuth{})
	gob.Register(&AzureSharedKeyAuth{})
}

// Copy returns a deep copy of the BackupRoutine.
func (r *BackupRoutine) Copy() *BackupRoutine {
	if r == nil {
		return nil
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(r); err != nil {
		panic(err) // if happens, registered failed types in init()
	}

	var out BackupRoutine
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		panic(err) // should never happen
	}

	return &out
}
