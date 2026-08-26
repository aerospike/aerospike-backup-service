package dto

import (
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
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

var timestampFormats = []TimestampFormat{TimestampFormatISO, TimestampFormatUS, TimestampFormatEU}

// Validate checks that the timestamp format is supported.
func (f TimestampFormat) Validate() error {
	if f == "" || slices.Contains(timestampFormats, f) {
		return nil
	}

	return errValidationInvalidValue("timestamp-format", f, timestampFormats)
}

// ToModel converts the DTO timestamp format to the model type.
func (f TimestampFormat) ToModel() *model.TimestampFormat {
	if f == "" {
		return nil
	}

	return ptr.Of(model.TimestampFormat(f))
}

// NewTimestampFormatFromModel creates a DTO timestamp format from the model type.
func NewTimestampFormatFromModel(m *model.TimestampFormat) TimestampFormat {
	if m == nil {
		return ""
	}

	return TimestampFormat(*m)
}
