package dto

import (
	"errors"
	"fmt"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// S3Storage represents the configuration for S3 storage.
// @Description S3Storage represents the configuration for S3 storage.
//
//nolint:lll
type S3Storage struct {
	SecretAgentConfig `yaml:",inline"`
	// The S3 bucket name.
	Bucket string `yaml:"bucket" json:"bucket" validate:"required"`
	// The root path for the backup repository within the bucket.
	// If not specified, backups will be saved in the bucket's root.
	Path string `yaml:"path,omitempty" json:"path,omitempty" example:"backups" extensions:"x-nullable"`
	// The S3 region string.
	S3Region string `yaml:"s3-region" json:"s3-region" example:"eu-central-1" validate:"required"`
	// The S3 profile name (AWS S3 optional).
	S3Profile string `yaml:"s3-profile,omitempty" json:"s3-profile,omitempty" example:"default" extensions:"x-nullable"`
	// An alternative endpoint for the S3 SDK to communicate (AWS S3 optional).
	S3EndpointOverride string `yaml:"s3-endpoint-override,omitempty" json:"s3-endpoint-override,omitempty" example:"http://host.docker.internal:9000" extensions:"x-nullable"`
	// The log level of the AWS S3 SDK (AWS S3 optional).
	S3LogLevel string `yaml:"s3-log-level,omitempty" json:"s3-log-level,omitempty" default:"FATAL" enums:"OFF,FATAL,ERROR,WARN,INFO,DEBUG,TRACE"`
	// The minimum size in bytes of individual S3 UploadParts.
	MinPartSize *int `yaml:"min-part-size,omitempty" json:"min-part-size,omitempty" default:"52428800" extensions:"x-nullable" minimum:"5242880"`
	// The maximum number of simultaneous requests from S3.
	MaxConnsPerHost *int `yaml:"max-async-connections,omitempty" json:"max-async-connections,omitempty" example:"16" extensions:"x-nullable"`
	// Access Key ID for authentication with S3 StaticCredentialsProvider.
	// This is sensitive information. Can be a path in secret agent or an actual value.
	// Literal values are redacted as "[secret]" in API responses; secret agent references are returned as-is.
	AccessKeyID secret `yaml:"access-key-id,omitempty" json:"access-key-id,omitempty" format:"password" extensions:"x-nullable"`
	// Secret Access Key for authentication with S3 StaticCredentialsProvider.
	// This is sensitive information. Can be a path in secret agent or an actual value.
	// Literal values are redacted as "[secret]" in API responses; secret agent references are returned as-is.
	SecretAccessKey secret `yaml:"secret-access-key,omitempty" json:"secret-access-key,omitempty" format:"password" extensions:"x-nullable"`
	// StorageClass defines the storage class for data and metadata objects.
	StorageClass *S3StorageClass `yaml:"storage-class,omitempty" json:"storage-class,omitempty"`
}

// s3MinUploadPartSize is the AWS-enforced minimum size of a multipart upload part (except the last one).
const s3MinUploadPartSize = 5 * 1024 * 1024 // 5 MiB

// Validate checks if the S3Storage is valid.
func (s *S3Storage) Validate(opts ...ValidationOption) error {
	if s.Bucket == "" {
		return errValidationEmptyField("bucket")
	}
	if s.S3Region == "" {
		return errValidationEmptyField("s3-region")
	}
	if err := validateObjectStoragePath(s.Path); err != nil {
		return err
	}

	if s.AccessKeyID != "" && s.SecretAccessKey == "" {
		return errors.New("access-key-id is set but secret-access-key is missing")
	}
	if s.AccessKeyID == "" && s.SecretAccessKey != "" {
		return errors.New("secret-access-key is set but access-key-id is missing")
	}
	if s.MinPartSize != nil && *s.MinPartSize < s3MinUploadPartSize {
		return errValidationInvalidValue("min-part-size", s.MinPartSize, "at least 5MiB")
	}
	if s.MaxConnsPerHost != nil && *s.MaxConnsPerHost < 0 {
		return errValidationNegative("max-async-connections", *s.MaxConnsPerHost)
	}
	if err := s.StorageClass.Validate(); err != nil {
		return fmt.Errorf("invalid storage class: %w", err)
	}
	//nolint:staticcheck // We want to call embedded methods with embedded struct name.
	return s.SecretAgentConfig.validate(opts...)
}

func (s *S3Storage) toModel(config *model.Config) (*model.S3Storage, error) {
	var auth *model.S3Authentication
	if s.AccessKeyID != "" {
		//nolint:staticcheck // We want to call embedded methods with embedded struct name.
		agent, err := s.SecretAgentConfig.ToModel(config)
		if err != nil {
			return nil, err
		}

		if err := validateSecretRef(s.AccessKeyID, agent); err != nil {
			return nil, fmt.Errorf("access-key-id: %w", err)
		}
		if err := validateSecretRef(s.SecretAccessKey, agent); err != nil {
			return nil, fmt.Errorf("secret-access-key: %w", err)
		}

		auth = &model.S3Authentication{
			KeyIDSecret:     string(s.AccessKeyID),
			AccessKeySecret: string(s.SecretAccessKey),
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
		result.AccessKeyID = secret(s.Auth.KeyIDSecret)
		result.SecretAccessKey = secret(s.Auth.AccessKeySecret)
	}

	return result
}

// Storage classes represents the different types of storage classes available on Amazon S3.
// See https://docs.aws.amazon.com/AmazonS3/latest/userguide/storage-class-intro.html for more details.
const (
	StorageClassStandard           = "STANDARD"
	StorageClassGlacier            = "GLACIER"
	StorageClassStandardIa         = "STANDARD_IA"
	StorageClassOnezoneIa          = "ONEZONE_IA"
	StorageClassIntelligentTiering = "INTELLIGENT_TIERING"
	StorageClassDeepArchive        = "DEEP_ARCHIVE"
	StorageClassOutposts           = "OUTPOSTS"
	StorageClassGlacierIr          = "GLACIER_IR"
	StorageClassSnow               = "SNOW"
	StorageClassExpressOnezone     = "EXPRESS_ONEZONE"
)

// metadata should only be stored in classes with fast retrieval time.
var s3metadataClasses = []string{
	"",
	StorageClassStandard,
	StorageClassStandardIa,
	StorageClassIntelligentTiering,
	StorageClassExpressOnezone,
	StorageClassOnezoneIa,
	StorageClassOutposts,
}

// backup data can be stored in any class.
var s3dataClasses = []string{
	"",
	StorageClassStandard,
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
//
//nolint:lll
type S3StorageClass struct {
	// DataClass specifies the storage class for object data.
	DataClass string `json:"data" yaml:"data" extensions:"x-nullable" enums:"STANDARD,GLACIER,STANDARD_IA,ONEZONE_IA,INTELLIGENT_TIERING,DEEP_ARCHIVE,OUTPOSTS,GLACIER_IR,SNOW,EXPRESS_ONEZONE"`

	// MetadataClass specifies the storage class for metadata.
	MetadataClass string `json:"metadata" yaml:"metadata" extensions:"x-nullable" enums:"STANDARD,STANDARD_IA,INTELLIGENT_TIERING,EXPRESS_ONEZONE,ONEZONE_IA,OUTPOSTS"`
}

func (s *S3StorageClass) Validate() error {
	if s == nil {
		return nil
	}

	if !slices.Contains(s3dataClasses, s.DataClass) {
		return errValidationInvalidValue("data", s.DataClass, s3dataClasses)
	}

	if !slices.Contains(s3metadataClasses, s.MetadataClass) {
		return errValidationInvalidValue("metadata", s.MetadataClass, s3metadataClasses)
	}

	return nil
}

func (s *S3StorageClass) ToModel() *model.StorageClass {
	if s == nil {
		return nil
	}

	return &model.StorageClass{
		DataClass:     s.DataClass,
		MetadataClass: s.MetadataClass,
	}
}

func newS3StorageClassFromModel(s *model.StorageClass) *S3StorageClass {
	if s == nil {
		return nil
	}

	return &S3StorageClass{
		DataClass:     s.DataClass,
		MetadataClass: s.MetadataClass,
	}
}
