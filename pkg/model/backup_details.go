package model

import (
	"fmt"
	"time"

	"github.com/aerospike/backup-go/models"
	"gopkg.in/yaml.v3"
)

// BackupDetails contains information about a backup.
type BackupDetails struct {
	BackupMetadata
	// The path to the backup files.
	Key     string
	Storage Storage
}

// BackupMetadata is an internal container for storing backup metadata.
// It is stored as a separate metadata file within each backup.
type BackupMetadata struct {
	// The backup time in the ISO 8601 format.
	Created time.Time `yaml:"created" json:"created"`
	// The lower time bound of backup entities in the ISO 8601 format (for incremental backups).
	From time.Time `yaml:"from" json:"from"`
	// The namespace of a backup.
	Namespace string `yaml:"namespace" json:"namespace"`
	// The total number of records backed up.
	RecordCount uint64 `yaml:"record-count" json:"record-count"`
	// The size of the backup in bytes.
	ByteCount uint64 `yaml:"byte-count" json:"byte-count"`
	// The number of backup files created.
	FileCount uint64 `yaml:"file-count" json:"file-count"`
	// The number of secondary indexes backed up.
	SecondaryIndexCount uint64 `yaml:"secondary-index-count" json:"secondary-index-count"`
	// The number of UDF files backed up.
	UDFCount uint64 `yaml:"udf-count" json:"udf-count"`
}

// NewMetadataFromBytes creates a new Metadata object from a byte slice
func NewMetadataFromBytes(data []byte) (*BackupMetadata, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty metadata file")
	}
	var metadata BackupMetadata
	err := yaml.Unmarshal(data, &metadata)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling YAML: %w", err)
	}
	return &metadata, nil
}

func NewMetadataFromStats(stats *models.BackupStats, namespace string, from, now time.Time) BackupMetadata {
	return BackupMetadata{
		From:                from,
		Created:             now,
		Namespace:           namespace,
		RecordCount:         stats.GetReadRecords(),
		FileCount:           stats.GetFileCount(),
		ByteCount:           stats.GetBytesWritten(),
		SecondaryIndexCount: uint64(stats.GetSIndexes()),
		UDFCount:            uint64(stats.GetUDFs()),
	}
}
