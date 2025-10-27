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
	TimestampFormat *string `yaml:"timestamp-format,omitempty" json:"timestamp-format,omitempty" enums:"ISO,US,EU" extensions:"x-nullable"` //nolint:lll
}

// Validate validates the backup subsection configuration.
func (c *BackupCommonConfig) Validate() error {
	if c == nil {
		return nil
	}

	if c.TimestampFormat != nil {
		format := model.TimestampFormatFromString(*c.TimestampFormat)
		if _, ok := model.TimestampFormatPresets[format]; !ok {
			allowed := slices.Collect(maps.Keys(model.TimestampFormatPresets))
			return errValidationInvalidValue("timestamp-format", *c.TimestampFormat, allowed)
		}
	}

	return nil
}

func (c *BackupCommonConfig) ToModel() *model.BackupCommonConfig {
	if c == nil {
		return nil
	}

	var df *model.TimestampFormat
	if c.TimestampFormat != nil {
		v := model.TimestampFormatFromString(*c.TimestampFormat)
		df = &v
	}

	return &model.BackupCommonConfig{TimestampFormat: df}
}

func (c *BackupCommonConfig) fromModel(m *model.BackupCommonConfig) {
	if m.TimestampFormat != nil {
		v := string(*m.TimestampFormat)
		c.TimestampFormat = &v
	}
}

// Compare compares two BackupCommonConfig instances and returns detailed errors.
func (c *BackupCommonConfig) Compare(other *BackupCommonConfig) error {
	if c == nil && other == nil {
		return nil
	}
	if c == nil {
		return errors.New("backup added")
	}
	if other == nil {
		return errors.New("backup removed")
	}
	return comparePointers("TimestampFormat", c.TimestampFormat, other.TimestampFormat)
}
