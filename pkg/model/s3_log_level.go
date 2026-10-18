package model

// S3LogLevel controls the verbosity of the AWS SDK logging.
type S3LogLevel string

// String returns the wire value of the S3 log level.
func (l S3LogLevel) String() string {
	return string(l)
}
