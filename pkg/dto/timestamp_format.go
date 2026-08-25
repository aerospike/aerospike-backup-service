package dto

import (
	"maps"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// TimestampFormat is the encoding for backup dates in file paths.
// @Description TimestampFormat is the encoding for backup dates in file paths.
type TimestampFormat string

const (
	// TimestampFormatISO is ISO-8601 style timestamps.
	TimestampFormatISO TimestampFormat = "ISO"
	// TimestampFormatUS is US-style timestamps.
	TimestampFormatUS TimestampFormat = "US"
	// TimestampFormatEU is EU-style timestamps.
	TimestampFormatEU TimestampFormat = "EU"
)

// Validate checks that the timestamp format is supported.
func (f TimestampFormat) Validate() error {
	if f == "" {
		return nil
	}
	if _, err := f.ToModel(); err != nil {
		return err
	}
	return nil
}

// ToModel converts the DTO timestamp format to the model type.
func (f TimestampFormat) ToModel() (*model.TimestampFormat, error) {
	if f == "" {
		return nil, nil
	}

	format := model.TimestampFormat(foldUpper(string(f)))
	if _, ok := model.TimestampFormatPresets[format]; !ok {
		allowed := slices.Collect(maps.Keys(model.TimestampFormatPresets))
		return nil, errValidationInvalidValue("timestamp-format", f, allowed)
	}

	return &format, nil
}

// NewTimestampFormatFromModel creates a DTO timestamp format from the model type.
func NewTimestampFormatFromModel(m *model.TimestampFormat) TimestampFormat {
	if m == nil {
		return ""
	}

	return TimestampFormat(*m)
}
