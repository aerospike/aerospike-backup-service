package dto

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/pkg/model"
)

// BackupDetails contains information about a backup.
// @Description BackupDetails contains information about a backup.
type BackupDetails struct {
	// The path to the backup files.
	Key *string `json:"key,omitempty" example:"storage/daily/backup/1707915600000/source-ns1"`
	// The backup time in the ISO 8601 format.
	Created *time.Time `json:"created,omitempty" example:"2023-03-20T14:50:00Z"`
	// The lower time bound of backup entities in the ISO 8601 format (for incremental backups).
	From *time.Time `json:"from,omitempty" example:"2023-03-19T14:50:00Z"`
	// The namespace of a backup.
	Namespace *string `json:"namespace,omitempty" example:"testNamespace"`
	// The total number of records backed up.
	RecordCount *uint64 `json:"record-count,omitempty" format:"int64" example:"100"`
	// The size of the backup in bytes.
	ByteCount *uint64 `json:"byte-count,omitempty" format:"int64" example:"2000"`
	// The number of backup files created.
	FileCount *uint64 `json:"file-count,omitempty" format:"int64" example:"1"`
	// The number of secondary indexes backed up.
	SecondaryIndexCount *uint64 `json:"secondary-index-count,omitempty" format:"int64" example:"5"`
	// The number of UDF files backed up.
	UDFCount *uint64 `json:"udf-count,omitempty" format:"int64" example:"2"`
}

func mapBackupDetailsToDTO(b model.BackupDetails) BackupDetails {
	return BackupDetails{
		Key:                 b.Key,
		Created:             &b.Created,
		From:                &b.From,
		Namespace:           &b.Namespace,
		RecordCount:         &b.RecordCount,
		ByteCount:           &b.ByteCount,
		FileCount:           &b.FileCount,
		SecondaryIndexCount: &b.SecondaryIndexCount,
		UDFCount:            &b.UDFCount,
	}
}

// MapBackupDetailsToDTOs maps slice of model.BackupDetails to slice of BackupDetails.
func MapBackupDetailsToDTOs(bs []model.BackupDetails) []BackupDetails {
	dtos := make([]BackupDetails, 0, len(bs))
	for i := range bs {
		dtos = append(dtos, mapBackupDetailsToDTO(bs[i]))
	}
	return dtos
}

// MapBackupDetailsMapsToDTOs maps map[string][]model.BackupDetails to map[string][]BackupDetails.
func MapBackupDetailsMapsToDTOs(m map[string][]model.BackupDetails) map[string][]BackupDetails {
	result := make(map[string][]BackupDetails, len(m))
	for k, v := range m {
		result[k] = MapBackupDetailsToDTOs(v)
	}
	return result
}
