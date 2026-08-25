package dto

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// LogFormat is the logger format.
// @Description LogFormat is the logger format.
type LogFormat string

const (
	LogFormatPlain LogFormat = LogFormat(model.LogFormatPlain)
	LogFormatJSON  LogFormat = LogFormat(model.LogFormatJSON)
)

func (f LogFormat) normalized() LogFormat {
	if f == "" {
		return f
	}
	return LogFormat(foldUpper(string(f)))
}

// Validate checks that the log format is supported.
func (f LogFormat) Validate() error {
	switch f.normalized() {
	case "", LogFormatPlain, LogFormatJSON:
		return nil
	default:
		return fmt.Errorf("invalid logger format: %s", f)
	}
}

// ToModel converts the DTO log format to the model type.
func (f LogFormat) ToModel() model.LogFormat {
	return model.LogFormat(f.normalized())
}

// NewLogFormatFromModel creates a DTO log format from the model type.
func NewLogFormatFromModel(m model.LogFormat) LogFormat {
	return LogFormat(m)
}
