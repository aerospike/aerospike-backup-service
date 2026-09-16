package model

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/backup-go/models"
)

// BackupDetails contains information about a backup.
type BackupDetails struct {
	BackupMetadata // Backup metadata that is stored in metadata.yaml file

	Key     string
	Storage Storage
}

func NewBackupDetails(md BackupMetadata, key string, storage Storage) BackupDetails {
	return BackupDetails{
		BackupMetadata: md,
		Key:            key,
		Storage:        storage,
	}
}

// BackupMetadata is an internal container for storing backup metadata.
// It is stored as a separate metadata file within each backup.
type BackupMetadata struct {
	// The backup time in the ISO 8601 format.
	Created time.Time `yaml:"created" json:"created"`
	// The time the backup operation completed.
	Finished time.Time `yaml:"finished" json:"finished"`
	// The lower time bound of backup entities in the ISO 8601 format (for incremental backups).
	// It's 0 for full backups.
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
	// Compression specifies the compression mode used for the backup (ZSTD or NONE).
	Compression CompressionMode
	// Encryption specifies the encryption mode used for the backup (NONE, AES128, AES256).
	Encryption EncryptionMode
}

// NewMetadataFromBytes creates a new Metadata object from a byte slice.
func NewMetadataFromBytes(data []byte) (*BackupMetadata, error) {
	if len(data) == 0 {
		return nil, errors.New("empty metadata file")
	}
	var metadata BackupMetadata
	if err := decoder.Deserialize(&metadata, bytes.NewReader(data), decoder.YAML); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	if err := metadata.Validate(); err != nil {
		return nil, fmt.Errorf("corrupted metadata: %w", err)
	}

	return &metadata, nil
}

func (m *BackupMetadata) Validate() error {
	if m.Created.IsZero() {
		return errors.New("`created` is required")
	}
	if m.Finished.IsZero() { // finished was introduced in ABS v3.4.0
		m.Finished = m.Created.Add(1 * time.Millisecond) // set dummy value
	}
	if m.Namespace == "" {
		return errors.New("`namespace` is required")
	}

	return nil
}

func NewBackupMetadata(
	stats *models.BackupStats,
	namespace string,
	from, startTime time.Time,
	backupPolicy *BackupPolicy,
) BackupMetadata {
	compression := CompressionModeNone
	if backupPolicy != nil && backupPolicy.CompressionPolicy != nil {
		compression = backupPolicy.CompressionPolicy.Mode
	}
	encryption := EncryptionModeNone
	if backupPolicy != nil && backupPolicy.EncryptionPolicy != nil {
		encryption = backupPolicy.EncryptionPolicy.Mode
	}
	return BackupMetadata{
		From:                from,
		Created:             startTime,
		Finished:            time.Now().Truncate(time.Millisecond), // ABS supports only millisecond accuracy.
		Namespace:           namespace,
		RecordCount:         stats.GetReadRecords(),
		FileCount:           stats.GetFileCount(),
		ByteCount:           stats.GetBytesWritten(),
		SecondaryIndexCount: uint64(stats.GetSIndexes()),
		UDFCount:            uint64(stats.GetUDFs()),
		Compression:         compression,
		Encryption:          encryption,
	}
}
