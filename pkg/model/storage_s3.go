package model

import (
	"fmt"
)

type S3Storage struct {
	// Path is the root directory within the S3 bucket where backups will be stored.
	// It should not include the bucket name.
	Path string
	// Bucket is the name of the S3 bucket where backups will be stored.
	Bucket string
	// S3Region is the AWS region where the S3 bucket is located.
	S3Region string
	// S3Profile is the name of the AWS credentials profile to use.
	S3Profile string
	// S3EndpointOverride is used to specify a custom S3 endpoint.
	S3EndpointOverride *string
	// S3LogLevel controls the verbosity of the AWS SDK logging.
	// Valid values are: OFF, FATAL, ERROR, WARN, INFO, DEBUG, TRACE.
	S3LogLevel *string
	// MinPartSize is the minimum size in bytes for multipart upload parts.
	MinPartSize *int
	// MaxConnsPerHost limits the number of concurrent connections to S3.
	MaxConnsPerHost *int
	// Optional authentication.
	Auth *S3Authentication
	// StorageClass defines the storage class for data and metadata objects.
	StorageClass *StorageClass
}

type S3Authentication struct {
	KeyIDSecret     string
	AccessKeySecret string
	SecretAgent     *SecretAgent
}

func (s *S3Storage) GetPath() string {
	return s.Path
}

func (s *S3Storage) String() string {
	return fmt.Sprintf("S3Storage(Bucket: %s, Path: %s)", s.Bucket, s.Path)
}

func (s *S3Storage) GetStorageClass() StorageClass {
	if s.StorageClass == nil {
		return StorageClass{}
	}

	return *s.StorageClass
}

func (s *S3Storage) GetPartSize() *int {
	return s.MinPartSize
}
