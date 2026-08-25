package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// S3LogLevel controls the verbosity of the AWS SDK logging.
// @Description S3LogLevel controls the verbosity of the AWS SDK logging.
type S3LogLevel string

const (
	S3LogLevelOff   S3LogLevel = "OFF"
	S3LogLevelFatal S3LogLevel = "FATAL"
	S3LogLevelError S3LogLevel = "ERROR"
	S3LogLevelWarn  S3LogLevel = "WARN"
	S3LogLevelInfo  S3LogLevel = "INFO"
	S3LogLevelDebug S3LogLevel = "DEBUG"
	S3LogLevelTrace S3LogLevel = "TRACE"
)

func (l S3LogLevel) normalized() S3LogLevel {
	if l == "" {
		return l
	}
	return S3LogLevel(foldUpper(string(l)))
}

// Validate checks that the S3 log level is supported.
func (l S3LogLevel) Validate() error {
	switch l.normalized() {
	case "", S3LogLevelOff, S3LogLevelFatal, S3LogLevelError, S3LogLevelWarn,
		S3LogLevelInfo, S3LogLevelDebug, S3LogLevelTrace:
		return nil
	default:
		return errValidationInvalidValue(
			"s3-log-level",
			l,
			[]S3LogLevel{S3LogLevelOff, S3LogLevelFatal, S3LogLevelError, S3LogLevelWarn,
				S3LogLevelInfo, S3LogLevelDebug, S3LogLevelTrace},
		)
	}
}

// ToModel converts the DTO S3 log level to the model type.
func (l S3LogLevel) ToModel() model.S3LogLevel {
	return model.S3LogLevel(l.normalized())
}

// NewS3LogLevelFromModel creates a DTO S3 log level from the model type.
func NewS3LogLevelFromModel(m model.S3LogLevel) S3LogLevel {
	return S3LogLevel(m)
}
