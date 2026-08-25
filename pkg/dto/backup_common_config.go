package dto

import (
	"errors"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// BackupCommonConfig represents service-level backup settings.
// @Description BackupCommonConfig represents service-level backup settings.
type BackupCommonConfig struct {
	// Encoding for backup date in human-readable format in backup file paths (optional).
	// Allowed values:
	// * ISO (e.g. 2006-01-02T15-04-05)
	// * EU (e.g. 02-Jan-2006-15-04-05)
	// * US (e.g. Jan-02-2006-15-04-05)
	TimestampFormat TimestampFormat `yaml:"timestamp-format,omitempty" json:"timestamp-format,omitempty" extensions:"x-nullable"` //nolint:lll
}

// Validate validates the backup subsection configuration.
func (b *BackupCommonConfig) Validate() error {
	if b == nil {
		return nil
	}

	return b.TimestampFormat.Validate()
}

func (b *BackupCommonConfig) ToModel() *model.BackupCommonConfig {
	if b == nil {
		return nil
	}

	format, _ := b.TimestampFormat.ToModel()
	return &model.BackupCommonConfig{TimestampFormat: format}
}

func (b *BackupCommonConfig) fromModel(m *model.BackupCommonConfig) {
	b.TimestampFormat = NewTimestampFormatFromModel(m.TimestampFormat)
}

// Compare compares two BackupCommonConfig instances and returns detailed errors.
func (b *BackupCommonConfig) Compare(other *BackupCommonConfig) error {
	if b == nil && other == nil {
		return nil
	}
	if b == nil {
		return errors.New("backup added")
	}
	if other == nil {
		return errors.New("backup removed")
	}
	return compareValues("TimestampFormat", b.TimestampFormat, other.TimestampFormat)
}
