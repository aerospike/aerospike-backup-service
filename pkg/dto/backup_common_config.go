package dto

import (
	"errors"
	"maps"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

const (
	BackupModeScan   = "scan"
	BackupModeServer = "server"
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
	// Backup mode for the entire service instance: scan (default) or server.
	BackupMode *string `yaml:"backup-mode,omitempty" json:"backup-mode,omitempty" enums:"scan,server" default:"scan" extensions:"x-nullable"`
}

// Validate validates the backup subsection configuration.
func (b *BackupCommonConfig) Validate() error {
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

	if b.BackupMode != nil &&
		*b.BackupMode != BackupModeScan && *b.BackupMode != BackupModeServer {
		return errValidationInvalidValue("backup-mode", *b.BackupMode, []string{BackupModeScan, BackupModeServer})
	}

	return nil
}

func (b *BackupCommonConfig) ToModel() *model.BackupCommonConfig {
	if b == nil {
		return nil
	}

	var df *model.TimestampFormat
	if b.TimestampFormat != nil {
		v := model.TimestampFormatFromString(*b.TimestampFormat)
		df = &v
	}

	var backupMode *model.BackupMode
	if b.BackupMode != nil {
		mode := model.BackupMode(*b.BackupMode)
		backupMode = &mode
	}

	return &model.BackupCommonConfig{
		TimestampFormat: df,
		BackupMode:      backupMode,
	}
}

func (b *BackupCommonConfig) fromModel(m *model.BackupCommonConfig) {
	if m.TimestampFormat != nil {
		v := string(*m.TimestampFormat)
		b.TimestampFormat = &v
	}
	if m.BackupMode != nil {
		mode := string(*m.BackupMode)
		b.BackupMode = &mode
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

	if err := comparePointers("TimestampFormat", b.TimestampFormat, other.TimestampFormat); err != nil {
		return err
	}

	return comparePointers("BackupMode", b.BackupMode, other.BackupMode)
}
