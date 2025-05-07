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
	// The time backup had finished.
	Finished time.Time `yaml:"finished" json:"finished" example:"2023-03-20T14:50:00Z"`
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
	// The path to the backup files in storage.
	Key string `yaml:"key" json:"key" example:"daily/backup/1707915600000/source-ns1"`
	// Storage specifying the data location.
	Storage *Storage `yaml:"storage" json:"storage"`
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
	d.Finished = m.Finished
	d.From = m.From
	d.Namespace = m.Namespace
	d.RecordCount = m.RecordCount
	d.ByteCount = m.ByteCount
	d.FileCount = m.FileCount
	d.SecondaryIndexCount = m.SecondaryIndexCount
	d.UDFCount = m.UDFCount
	d.Storage = NewStorageFromModel(m.Storage, config)
}

func ConvertBackupDetailsMap(
	modelMap map[string][]model.BackupDetails, config *model.BackupConfig,
) map[string][]BackupDetails {
	result := make(map[string][]BackupDetails, len(modelMap))
	for key, modelSlice := range modelMap {
		dtoSlice := make([]BackupDetails, len(modelSlice))
		for i := range modelSlice {
			dtoSlice[i] = *NewBackupDetailsFromModel(&modelSlice[i], config)
		}
		result[key] = dtoSlice
	}
	return result
}
