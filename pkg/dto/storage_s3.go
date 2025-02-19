package dto

import (
	"fmt"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Storage represents the configuration for S3 storage.
//
//nolint:lll
type S3Storage struct {
	SecretAgentConfig `yaml:",inline"`
	// The S3 bucket name.
	Bucket string `yaml:"bucket" json:"bucket" validate:"required"`
	// The root path for the backup repository within the bucket.
	// If not specified, backups will be saved in the bucket's root.
	Path string `yaml:"path,omitempty" json:"path,omitempty" example:"backups"`
	// The S3 region string.
	S3Region string `yaml:"s3-region" json:"s3-region" example:"eu-central-1" validate:"required"`
	// The S3 profile name (AWS S3 optional).
	S3Profile string `yaml:"s3-profile,omitempty" json:"s3-profile,omitempty" example:"default"`
	// An alternative endpoint for the S3 SDK to communicate (AWS S3 optional).
	S3EndpointOverride *string `yaml:"s3-endpoint-override,omitempty" json:"s3-endpoint-override,omitempty" example:"http://host.docker.internal:9000"`
	// The log level of the AWS S3 SDK (AWS S3 optional).
	S3LogLevel *string `yaml:"s3-log-level,omitempty" json:"s3-log-level,omitempty" default:"FATAL" enum:"OFF,FATAL,ERROR,WARN,INFO,DEBUG,TRACE"`
	// The minimum size in bytes of individual S3 UploadParts.
	MinPartSize int `yaml:"min-part-size,omitempty" json:"min-part-size,omitempty" example:"10" default:"5242880"`
	// The maximum number of simultaneous requests from S3.
	MaxConnsPerHost int `yaml:"max-async-connections,omitempty" json:"max-async-connections,omitempty" example:"16"`
	// Access Key ID for authentication with S3 StaticCredentialsProvider.
	// This is sensitive information. Can be a path in secret agent or an actual value.
	AccessKeyID *string `yaml:"access-key-id,omitempty" json:"access-key-id,omitempty"`
	// Secret Access Key for authentication with S3 StaticCredentialsProvider.
	// This is sensitive information. Can be a path in secret agent or an actual value.
	SecretAccessKey *string `yaml:"secret-access-key,omitempty" json:"secret-access-key,omitempty"`
	// StorageClass defines the storage class for data and metadata objects.
	StorageClass *S3StorageClass `yaml:"storage-class,omitempty" json:"storage-class,omitempty"`
}

// Validate checks if the S3Storage is valid.
func (s *S3Storage) Validate() error {
	if s.Bucket == "" {
		return errValidationEmptyField("bucket")
	}
	if s.S3Region == "" {
		return errValidationEmptyField("s3-region")
	}

	if s.AccessKeyID != nil && s.SecretAccessKey == nil {
		return fmt.Errorf("access-key-id is set but secret-access-key is missing")
	}
	if s.AccessKeyID == nil && s.SecretAccessKey != nil {
		return fmt.Errorf("secret-access-key is set but access-key-id is missing")
	}
	if err := s.StorageClass.Validate(); err != nil {
		return fmt.Errorf("invalid storage class: %w", err)
	}

	return s.SecretAgentConfig.validate()
}

func (s *S3Storage) toModel(config *model.Config) (*model.S3Storage, error) {
	var auth *model.S3Authentication
	if s.AccessKeyID != nil {
		agent, err := s.SecretAgentConfig.ToModel(config)
		if err != nil {
			return nil, err
		}

		auth = &model.S3Authentication{
			KeyIDSecret:     *s.AccessKeyID,
			AccessKeySecret: *s.SecretAccessKey,
			SecretAgent:     agent,
		}
	}

	return &model.S3Storage{
		Path:               s.Path,
		Bucket:             s.Bucket,
		S3Region:           s.S3Region,
		S3Profile:          s.S3Profile,
		S3EndpointOverride: s.S3EndpointOverride,
		S3LogLevel:         s.S3LogLevel,
		MinPartSize:        s.MinPartSize,
		MaxConnsPerHost:    s.MaxConnsPerHost,
		Auth:               auth,
		StorageClass:       s.StorageClass.ToModel(),
	}, nil
}

func newS3StorageFromModel(s *model.S3Storage, config *model.BackupConfig) *S3Storage {
	result := &S3Storage{
		Bucket:             s.Bucket,
		Path:               s.Path,
		S3Region:           s.S3Region,
		S3Profile:          s.S3Profile,
		S3EndpointOverride: s.S3EndpointOverride,
		S3LogLevel:         s.S3LogLevel,
		MinPartSize:        s.MinPartSize,
		MaxConnsPerHost:    s.MaxConnsPerHost,
		StorageClass:       newS3StorageClassFromModel(s.StorageClass),
	}
	if s.Auth != nil {
		result.SecretAgentConfig = ResolveSecretAgentFromModel(s.Auth.SecretAgent, config)
		result.AccessKeyID = &s.Auth.KeyIDSecret
		result.SecretAccessKey = &s.Auth.AccessKeySecret
	}

	return result
}

// StorageClass represents the different types of storage classes available on Amazon S3.
// See https://docs.aws.amazon.com/AmazonS3/latest/userguide/storage-class-intro.html for more details.
// @Description Storage classes available on Amazon S3
type StorageClass string

const (
	StorageClassStandard           StorageClass = "STANDARD"
	StorageClassReducedRedundancy  StorageClass = "REDUCED_REDUNDANCY"
	StorageClassGlacier            StorageClass = "GLACIER"
	StorageClassStandardIa         StorageClass = "STANDARD_IA"
	StorageClassOnezoneIa          StorageClass = "ONEZONE_IA"
	StorageClassIntelligentTiering StorageClass = "INTELLIGENT_TIERING"
	StorageClassDeepArchive        StorageClass = "DEEP_ARCHIVE"
	StorageClassOutposts           StorageClass = "OUTPOSTS"
	StorageClassGlacierIr          StorageClass = "GLACIER_IR"
	StorageClassSnow               StorageClass = "SNOW"
	StorageClassExpressOnezone     StorageClass = "EXPRESS_ONEZONE"
)

// metadata should only be stored in classes with fast retrieval time.
var s3metadataClasses = []StorageClass{
	StorageClassStandard,
	StorageClassIntelligentTiering,
	StorageClassExpressOnezone,
}

// backup data can be stored in any class.
var s3dataClasses = []StorageClass{
	StorageClassStandard,
	StorageClassReducedRedundancy,
	StorageClassGlacier,
	StorageClassStandardIa,
	StorageClassOnezoneIa,
	StorageClassIntelligentTiering,
	StorageClassDeepArchive,
	StorageClassOutposts,
	StorageClassGlacierIr,
	StorageClassSnow,
	StorageClassExpressOnezone,
}

// S3StorageClass represents the configuration for S3 Storage Class.
// @Description S3StorageClass represents the configuration for S3 Storage Class.
type S3StorageClass struct {
	// DataClass specifies the storage class for object data
	DataClass *StorageClass `json:"data" yaml:"data"`

	// MetadataClass specifies the storage class for metadata
	MetadataClass *StorageClass `json:"metadata" yaml:"metadata"`
}

func (s *S3StorageClass) Validate() error {
	if s == nil {
		return nil
	}

	if s.DataClass != nil && !slices.Contains(s3dataClasses, *s.DataClass) {
		return errValidationInvalidValue("data", s.DataClass, s3dataClasses)
	}

	if s.MetadataClass != nil && !slices.Contains(s3metadataClasses, *s.MetadataClass) {
		return errValidationInvalidValue("metadata", s.MetadataClass, s3metadataClasses)
	}

	return nil
}

func (s *S3StorageClass) ToModel() *model.S3StorageClass {
	if s == nil {
		return nil
	}

	return &model.S3StorageClass{
		DataClass:     (*types.StorageClass)(s.DataClass),
		MetadataClass: (*types.StorageClass)(s.MetadataClass),
	}
}

func newS3StorageClassFromModel(s *model.S3StorageClass) *S3StorageClass {
	if s == nil {
		return nil
	}

	return &S3StorageClass{
		DataClass:     (*StorageClass)(s.DataClass),
		MetadataClass: (*StorageClass)(s.MetadataClass),
	}
}
