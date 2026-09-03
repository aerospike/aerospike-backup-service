package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// LogFormat is the logger format.
// @Description LogFormat is the logger format.
type LogFormat string

const (
	LogFormatPlain LogFormat = "PLAIN"
	LogFormatJSON  LogFormat = "JSON"
)

var logFormats = []LogFormat{LogFormatPlain, LogFormatJSON}

// Validate checks that the log format is supported.
func (f LogFormat) Validate() error {
	if _, ok := canonicalEnum(f, logFormats); ok {
		return nil
	}

	return errValidationInvalidValue("format", f, logFormats)
}

// ToModel converts the DTO log format to the model type.
func (f LogFormat) ToModel() model.LogFormat {
	c, _ := canonicalEnum(f, logFormats)
	return model.LogFormat(c)
}

// NewLogFormatFromModel creates a DTO log format from the model type.
func NewLogFormatFromModel(m model.LogFormat) LogFormat {
	return LogFormat(m)
}
