package dto

import (
	"errors"
	"maps"
	"slices"

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
	TimestampFormat string `yaml:"timestamp-format,omitempty" json:"timestamp-format,omitempty" enums:"ISO,US,EU" extensions:"x-nullable"` //nolint:lll
}

// Validate validates the backup subsection configuration.
func (b *BackupCommonConfig) Validate() error {
	if b == nil {
		return nil
	}

	if b.TimestampFormat != "" {
		format := model.TimestampFormatFromString(b.TimestampFormat)
		if _, ok := model.TimestampFormatPresets[format]; !ok {
			allowed := slices.Collect(maps.Keys(model.TimestampFormatPresets))
			return errValidationInvalidValue("timestamp-format", b.TimestampFormat, allowed)
		}
	}

	return nil
}

func (b *BackupCommonConfig) ToModel() *model.BackupCommonConfig {
	if b == nil {
		return nil
	}

	var df *model.TimestampFormat
	if b.TimestampFormat != "" {
		v := model.TimestampFormatFromString(b.TimestampFormat)
		df = &v
	}

	return &model.BackupCommonConfig{TimestampFormat: df}
}

func (b *BackupCommonConfig) fromModel(m *model.BackupCommonConfig) {
	if m.TimestampFormat != nil {
		b.TimestampFormat = string(*m.TimestampFormat)
	}
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
