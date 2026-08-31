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
	// Timezone for evaluating backup cron expressions (optional).
	// Accepted values: UTC (default), Local, or an IANA timezone name such as America/New_York.
	// Keywords UTC and Local are case-insensitive; IANA names are case-sensitive.
	// Abbreviations such as EST and POSIX TZ strings are not accepted.
	// Changing this service-level default requires a restart.
	ScheduleTimezone string `yaml:"schedule-timezone,omitempty" json:"schedule-timezone,omitempty" example:"America/New_York" extensions:"x-nullable"` //nolint:lll
}

// Validate validates the backup subsection configuration.
func (b *BackupCommonConfig) Validate() error {
	if b == nil {
		return nil
	}

	if err := b.TimestampFormat.Validate(); err != nil {
		return err
	}

	if _, err := model.ParseScheduleTimezone(b.ScheduleTimezone); err != nil {
		return err
	}

	return nil
}

func (b *BackupCommonConfig) ToModel() *model.BackupCommonConfig {
	if b == nil {
		return nil
	}

	return &model.BackupCommonConfig{
		TimestampFormat:  b.TimestampFormat.ToModel(),
		ScheduleTimezone: b.ScheduleTimezone,
	}
}

func (b *BackupCommonConfig) fromModel(m *model.BackupCommonConfig) {
	b.TimestampFormat = NewTimestampFormatFromModel(m.TimestampFormat)
	b.ScheduleTimezone = m.ScheduleTimezone
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
	return errors.Join(
		compareValues("TimestampFormat", b.TimestampFormat, other.TimestampFormat),
		compareValues("ScheduleTimezone", b.ScheduleTimezone, other.ScheduleTimezone),
	)
}
