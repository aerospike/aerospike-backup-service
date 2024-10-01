package model

// Storage represents the configuration for a backup storage details.
type Storage interface {
	storage()
}

type LocalStorage struct {
	// The root path for the backup repository.
	Path string
}

func (s *LocalStorage) storage() {}

type S3Storage struct {
	// The root path for the backup repository (just a path, without bucket)
	Path string
	// The S3 bucket
	Bucket string
	// The S3 region string (AWS S3 optional).
	S3Region string
	// The S3 profile name (AWS S3 optional).
	S3Profile string
	// An alternative endpoint for the S3 SDK to communicate (AWS S3 optional).
	S3EndpointOverride *string
	// The log level of the AWS S3 SDK (AWS S3 optional).
	S3LogLevel *string
	// The minimum size in bytes of individual S3 UploadParts
	MinPartSize int
	// The maximum number of simultaneous requests from S3.
	MaxConnsPerHost int
}

func (s *S3Storage) storage() {}

type GcpStorage struct {
	// Path to file containing Service Account JSON Key.
	KeyFile string
	// For GCP storage bucket is not part of the path as in S3.
	// So we should set it separately.
	BucketName string
	// The root path for the backup repository.
	Path string
	// Alternative url.
	// It is not recommended to use an alternate URL in a production environment.
	Endpoint string
}

func (s *GcpStorage) storage() {}
