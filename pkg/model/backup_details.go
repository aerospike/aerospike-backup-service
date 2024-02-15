package model

import (
	"encoding/json"
	"time"
)

// BackupDetails contains information about a backup.
// @Description BackupDetails contains information about a backup.
type BackupDetails struct {
	BackupMetadata
	// The path to the backup files.
	Key *string `yaml:"key,omitempty" json:"key,omitempty" example:"storage/daily/backup/1707915600000/source-ns1"`
}

// String satisfies the fmt.Stringer interface.
func (details BackupDetails) String() string {
	backupDetails, err := json.Marshal(details)
	if err != nil {
		return err.Error()
	}
	return string(backupDetails)
}

// BackupMetadata is an internal container for storing backup metadata.
type BackupMetadata struct {
	// The backup time in the ISO 8601 format.
	Created time.Time `yaml:"created,omitempty" json:"created,omitempty" example:"2023-03-20T14:50:00Z"`
	// The lower time bound of backup entities in the ISO 8601 format (for incremental backups).
	From time.Time `yaml:"from,omitempty" json:"from,omitempty" example:"2023-03-19T14:50:00Z"`
	// The namespace of a backup.
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty" example:"testNamespace"`
	// The total number of records backed up.
	RecordCount int `yaml:"record-count,omitempty" json:"record-count,omitempty" example:"100"`
	// The size of the backup in bytes.
	ByteCount int `yaml:"byte-count,omitempty" json:"byte-count,omitempty" example:"2000"`
	// The number of backup files created.
	FileCount int `yaml:"file-count,omitempty" json:"file-count,omitempty" example:"1"`
	// The number of secondary indexes backed up.
	SecondaryIndexCount int `yaml:"secondary-index-count,omitempty" json:"secondary-index-count,omitempty" example:"5"`
	// The number of UDF files backed up.
	UDFCount int `yaml:"udf-count,omitempty" json:"udf-count,omitempty" example:"2"`
}
