package model

import (
	"time"
)

// BackupDetails contains information about a backup.
// @Description BackupDetails contains information about a backup.
type BackupDetails struct {
	BackupMetadata
	// The path to the backup files.
	Key *string
}

// BackupMetadata is an internal container for storing backup metadata.
//

type BackupMetadata struct {
	// The backup time in the ISO 8601 format.
	Created time.Time
	// The lower time bound of backup entities in the ISO 8601 format (for incremental backups).
	From time.Time
	// The namespace of a backup.
	Namespace string
	// The total number of records backed up.
	RecordCount uint64
	// The size of the backup in bytes.
	ByteCount uint64
	// The number of backup files created.
	FileCount uint64
	// The number of secondary indexes backed up.
	SecondaryIndexCount uint64
	// The number of UDF files backed up.
	UDFCount uint64
}
