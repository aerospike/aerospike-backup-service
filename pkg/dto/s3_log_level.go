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

var s3LogLevels = []S3LogLevel{
	S3LogLevelOff,
	S3LogLevelFatal,
	S3LogLevelError,
	S3LogLevelWarn,
	S3LogLevelInfo,
	S3LogLevelDebug,
	S3LogLevelTrace,
}

// Validate checks that the S3 log level is supported.
func (l S3LogLevel) Validate() error {
	if _, ok := canonicalEnum(l, s3LogLevels); ok {
		return nil
	}

	return errValidationInvalidValue("s3-log-level", l, s3LogLevels)
}

// ToModel converts the DTO S3 log level to the model type.
func (l S3LogLevel) ToModel() model.S3LogLevel {
	c, _ := canonicalEnum(l, s3LogLevels)
	return model.S3LogLevel(c)
}

// NewS3LogLevelFromModel creates a DTO S3 log level from the model type.
func NewS3LogLevelFromModel(m model.S3LogLevel) S3LogLevel {
	return S3LogLevel(m)
}
