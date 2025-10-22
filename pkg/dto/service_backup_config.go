package dto

import (
	"errors"
	"maps"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// ServiceBackupConfig represents service-level backup settings.
// @Description ServiceBackupConfig represents service-level backup settings.
type ServiceBackupConfig struct {
	// Encoding for backup date in human-readable format in backup file paths (optional).
	// Allowed values:
	// * ISO (e.g. 2006-01-02T15-04-05)
	// * EU (e.g. 02-Jan-2006-15-04-05)
	// * US (e.g. Jan-02-2006-15-04-05)
	TimestampFormat *string `yaml:"timestamp-format,omitempty" json:"timestamp-format,omitempty" enums:"ISO,US,EU" extensions:"x-nullable"` //nolint:lll
}

// Validate validates the backup subsection configuration.
func (b *ServiceBackupConfig) Validate() error {
	if b == nil {
		return nil
	}

	if b.TimestampFormat != nil {
		format := model.TimestampFormatFromString(*b.TimestampFormat)
		if _, ok := model.TimestampFormatPresets[format]; !ok {
			allowed := slices.Collect(maps.Keys(model.TimestampFormatPresets))
			return errValidationInvalidValue("timestamp-format", *b.TimestampFormat, allowed)
		}
	}

	return nil
}

func (b *ServiceBackupConfig) ToModel() *model.ServiceBackupConfig {
	if b == nil {
		return nil
	}

	var df *model.TimestampFormat
	if b.TimestampFormat != nil {
		v := model.TimestampFormatFromString(*b.TimestampFormat)
		df = &v
	}

	return &model.ServiceBackupConfig{TimestampFormat: df}
}

func (b *ServiceBackupConfig) fromModel(m *model.ServiceBackupConfig) {
	if m.TimestampFormat != nil {
		v := string(*m.TimestampFormat)
		b.TimestampFormat = &v
	}
}

// Compare compares two ServiceBackupConfig instances and returns detailed errors.
func (b *ServiceBackupConfig) Compare(other *ServiceBackupConfig) error {
	if b == nil && other == nil {
		return nil
	}
	if b == nil {
		return errors.New("backup added")
	}
	if other == nil {
		return errors.New("backup removed")
	}
	return comparePointers("TimestampFormat", b.TimestampFormat, other.TimestampFormat)
}
