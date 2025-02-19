package model

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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
	MinPartSize int
	// MaxConnsPerHost limits the number of concurrent connections to S3.
	MaxConnsPerHost int
	// Optional authentication.
	Auth *S3Authentication
	// StorageClass defines the storage class for data and metadata objects.
	StorageClass *S3StorageClass
}

type S3Authentication struct {
	KeyIDSecret     string
	AccessKeySecret string
	SecretAgent     *SecretAgent
}

func (a *S3Authentication) ReadSecrets() (keyID, accessKey string, err error) {
	if a.SecretAgent == nil {
		return a.KeyIDSecret, a.AccessKeySecret, nil
	}

	keyID, err = a.SecretAgent.Read(a.KeyIDSecret)
	if err != nil {
		return
	}

	accessKey, err = a.SecretAgent.Read(a.AccessKeySecret)
	if err != nil {
		return
	}

	return
}

func (s *S3Storage) GetPath() string {
	return s.Path
}
func (s *S3Storage) String() string {
	return fmt.Sprintf("S3Storage(Bucket: %s, Path: %s)", s.Bucket, s.Path)
}

type S3StorageClass struct {
	DataClass     *types.ObjectStorageClass
	MetadataClass *types.ObjectStorageClass
}
