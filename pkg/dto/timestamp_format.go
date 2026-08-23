package dto

import (
	"strings"

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

// Validate checks that the timestamp format is supported.
func (f TimestampFormat) Validate() error {
	_, err := f.ToModel()
	return err
}

// ToModel converts the DTO timestamp format to the model type.
func (f TimestampFormat) ToModel() (*model.TimestampFormat, error) {
	if f == "" {
		return nil, nil
	}

	switch TimestampFormat(strings.ToUpper(strings.TrimSpace(string(f)))) {
	case TimestampFormatISO:
		return ptr.Of(model.TimestampFormatISO), nil
	case TimestampFormatUS:
		return ptr.Of(model.TimestampFormatUS), nil
	case TimestampFormatEU:
		return ptr.Of(model.TimestampFormatEU), nil
	default:
		return nil, errValidationInvalidValue(
			"timestamp-format",
			f,
			[]TimestampFormat{TimestampFormatISO, TimestampFormatUS, TimestampFormatEU},
		)
	}
}

// NewTimestampFormatFromModel creates a DTO timestamp format from the model type.
func NewTimestampFormatFromModel(m *model.TimestampFormat) TimestampFormat {
	if m == nil {
		return ""
	}

	return TimestampFormat(*m)
}
