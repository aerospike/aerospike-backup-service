package model

import (
	"bytes"
	"encoding/gob"
	"time"
)

// BackupRoutine represents a scheduled backup operation routine.
type BackupRoutine struct {
	// The unique name of the routine (key in routines map).
	Name string

	// The corresponding backup policy. Not nil.
	BackupPolicy *BackupPolicy
	// The corresponding source cluster. Not nil.
	SourceCluster *AerospikeCluster
	// The corresponding storage provider configuration. Not nil.
	Storage Storage
	// The Secret Agent configuration for the routine (optional).
	SecretAgent *SecretAgent
	// The interval for full backup as a cron expression string.
	IntervalCron string
	// The interval for incremental backup as a cron expression string (optional).
	IncrIntervalCron string
	// Timezone is the resolved timezone for evaluating cron expressions. Not nil
	// after DTO conversion. Inherited from service.backup.schedule-timezone when
	// ConfiguredTimezone is empty.
	Timezone *time.Location
	// ConfiguredTimezone is this routine's own canonical schedule-timezone.
	// Empty means the routine inherits the service default
	ConfiguredTimezone string
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
	// Base64 encoded filter expression used in each scan call for partial backup.
	FilterExpression string
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

// backupRoutineGob is BackupRoutine without Timezone. gob cannot encode
// time.Location (no exported fields), so Copy() round-trips through this type.
type backupRoutineGob struct {
	Name               string
	BackupPolicy       *BackupPolicy
	SourceCluster      *AerospikeCluster
	Storage            Storage
	SecretAgent        *SecretAgent
	IntervalCron       string
	IncrIntervalCron   string
	ConfiguredTimezone string
	Namespaces         []string
	SetList            []string
	BinList            []string
	RackList           []int
	PartitionList      string
	NodeList           []string
	FilterExpression   string
	Disabled           bool
}

func toBackupRoutineGob(r *BackupRoutine) backupRoutineGob {
	return backupRoutineGob{
		Name:               r.Name,
		BackupPolicy:       r.BackupPolicy,
		SourceCluster:      r.SourceCluster,
		Storage:            r.Storage,
		SecretAgent:        r.SecretAgent,
		IntervalCron:       r.IntervalCron,
		IncrIntervalCron:   r.IncrIntervalCron,
		ConfiguredTimezone: r.ConfiguredTimezone,
		Namespaces:         r.Namespaces,
		SetList:            r.SetList,
		BinList:            r.BinList,
		RackList:           r.RackList,
		PartitionList:      r.PartitionList,
		NodeList:           r.NodeList,
		FilterExpression:   r.FilterExpression,
		Disabled:           r.Disabled,
	}
}

// Copy returns a deep copy of the BackupRoutine.
// Long-running backup/restore operations must work on an immutable routine snapshot.
// A shallow copy would still share nested pointers/slices and could observe config changes mid-run.
func (r *BackupRoutine) Copy() *BackupRoutine {
	if r == nil {
		return nil
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(toBackupRoutineGob(r)); err != nil {
		panic(err) // if happens, registered failed types in init()
	}

	var copied backupRoutineGob
	if err := gob.NewDecoder(&buf).Decode(&copied); err != nil {
		panic(err) // should never happen
	}

	return &BackupRoutine{
		Name:               copied.Name,
		BackupPolicy:       copied.BackupPolicy,
		SourceCluster:      copied.SourceCluster,
		Storage:            copied.Storage,
		SecretAgent:        copied.SecretAgent,
		IntervalCron:       copied.IntervalCron,
		IncrIntervalCron:   copied.IncrIntervalCron,
		Timezone:           r.Timezone, // immutable; shared pointer is safe
		ConfiguredTimezone: copied.ConfiguredTimezone,
		Namespaces:         copied.Namespaces,
		SetList:            copied.SetList,
		BinList:            copied.BinList,
		RackList:           copied.RackList,
		PartitionList:      copied.PartitionList,
		NodeList:           copied.NodeList,
		FilterExpression:   copied.FilterExpression,
		Disabled:           copied.Disabled,
	}
}
