package dto

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// BackupDetails contains information about a backup.
// @Description BackupDetails contains information about a backup.
type BackupDetails struct {
	// The backup time in the ISO 8601 format.
	Created time.Time `yaml:"created" json:"created" example:"2023-03-20T14:50:00Z"`
	// The backup time in epoch millis.
	Timestamp int64 `yaml:"timestamp" json:"timestamp" example:"1685458200000" format:"int64"`
	// The time the backup operation completed.
	Finished time.Time `yaml:"finished" json:"finished" example:"2023-03-20T14:50:00Z"`
	// DurationSec represents the elapsed time taken by the backup process in seconds.
	DurationSec uint `yaml:"duration" json:"duration"`
	// The lower time bound of backup entities in the ISO 8601 format (for incremental backups only).
	From time.Time `yaml:"from,omitempty" json:"from,omitempty" example:"2023-03-19T14:50:00Z"`
	// The namespace of a backup.
	Namespace string `yaml:"namespace" json:"namespace" example:"testNamespace"`
	// The total number of records backed up.
	RecordCount uint64 `yaml:"record-count" json:"record-count" format:"int64" example:"100"`
	// The size of the backup in bytes.
	ByteCount uint64 `yaml:"byte-count" json:"byte-count" format:"int64" example:"2000"`
	// The number of backup files created.
	FileCount uint64 `yaml:"file-count" json:"file-count" format:"int64" example:"1"`
	// The number of secondary indexes backed up.
	SecondaryIndexCount uint64 `yaml:"secondary-index-count" json:"secondary-index-count" format:"int64" example:"5"`
	// The number of UDF files backed up.
	UDFCount uint64 `yaml:"udf-count" json:"udf-count" format:"int64" example:"2"`
	// Key is the path to the backup files within the configured storage location.
	// This values is used as `backup-data-path` in dto.RestoreRequest
	Key string `yaml:"key" json:"key" example:"daily/backup/1707915600000/source-ns1"`
	// Storage specifies the details of the storage location where the backup is stored.
	Storage *Storage `yaml:"storage" json:"storage"`
	// Compression specifies the compression mode used for the backup (ZSTD or NONE).
	Compression string `yaml:"compression" json:"compression"`
	// Encryption specifies the encryption mode used for the backup (NONE, AES128, AES256).
	Encryption string `yaml:"encryption" json:"encryption"`
}

// NewBackupDetailsFromModel creates a new BackupDetails from a model.BackupDetails.
func NewBackupDetailsFromModel(m *model.BackupDetails, config *model.BackupConfig) *BackupDetails {
	if m == nil {
		return nil
	}

	var d BackupDetails
	d.fromModel(m, config)
	return &d
}

func (d *BackupDetails) fromModel(m *model.BackupDetails, config *model.BackupConfig) {
	d.Key = m.Key
	d.Created = m.Created
	d.Timestamp = m.Created.UnixMilli()
	d.Finished = m.Finished
	d.DurationSec = uint(m.Finished.Sub(d.Created) / time.Second)
	d.From = m.From
	d.Namespace = m.Namespace
	d.RecordCount = m.RecordCount
	d.ByteCount = m.ByteCount
	d.FileCount = m.FileCount
	d.SecondaryIndexCount = m.SecondaryIndexCount
	d.UDFCount = m.UDFCount
	d.Encryption = m.Encryption
	d.Compression = m.Compression
	d.Storage = NewStorageFromModel(m.Storage, config)
}
