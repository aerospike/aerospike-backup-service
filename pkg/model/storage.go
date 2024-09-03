package model

import "github.com/aws/smithy-go/ptr"

// Storage represents the configuration for a backup storage details.
// @Description Storage represents the configuration for a backup storage details.
//

type Storage struct {
	// The type of the storage provider
	Type StorageType
	// The root path for the backup repository.
	Path *string
	// The S3 region string (AWS S3 optional).
	S3Region *string
	// The S3 profile name (AWS S3 optional).
	S3Profile *string
	// An alternative endpoint for the S3 SDK to communicate (AWS S3 optional).
	S3EndpointOverride *string
	// The log level of the AWS S3 SDK (AWS S3 optional).
	S3LogLevel *string
	// The minimum size in bytes of individual S3 UploadParts
	MinPartSize int
	// The maximum number of simultaneous requests from S3.
	MaxConnsPerHost int
}

// StorageType represents the type of the backup storage.
// @Description StorageType represents the type of the backup storage.
type StorageType string

const (
	Local StorageType = "local"
	S3    StorageType = "aws-s3"
)

// SetDefaultProfile sets the "default" profile if not set.
func (s *Storage) SetDefaultProfile() {
	if s.Type == S3 && s.S3Profile == nil {
		s.S3Profile = ptr.String("default")
	}
}
