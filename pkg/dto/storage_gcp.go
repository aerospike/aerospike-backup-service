package dto

import (
	"errors"
	"fmt"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// GcpStorage represents the configuration for GCP storage.
// @Description GcpStorage represents the configuration for GCP storage.
type GcpStorage struct {
	SecretAgentConfig `yaml:",inline"`
	// Path to the file containing the service account key in JSON format.
	KeyFile string `yaml:"key-file-path,omitempty" json:"key-file-path,omitempty" extensions:"x-nullable"`
	// Key is the service account key in JSON format.
	// This is sensitive information. Can be a path in secret agent or an actual value.
	// Literal values are redacted as "[secret]" in API responses; secret agent references are returned as-is.
	Key secret `yaml:"key,omitempty" json:"key,omitempty" format:"password" extensions:"x-nullable"`
	// GCP storage bucket name.
	BucketName string `yaml:"bucket-name" json:"bucket-name" validate:"required"`
	// The root path for the backup repository. If not specified, backups will be saved in the bucket's root.
	Path string `yaml:"path,omitempty" json:"path,omitempty" example:"backups" extensions:"x-nullable"`
	// Alternative url.
	// It is not recommended to use an alternate URL in a production environment.
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty" extensions:"x-nullable"`
	// The minimum size in bytes of individual GCP storage chunks.
	MinPartSize *int `yaml:"min-part-size,omitempty" json:"min-part-size,omitempty" default:"52428800" minimum:"262144"`
	// StorageClass defines the storage class for data and metadata objects.
	StorageClass *GcpStorageClass `yaml:"storage-class,omitempty" json:"storage-class,omitempty" extensions:"x-nullable"`
}

// gcsMinUploadChunkSize minimum size of multipart upload.
// see https://cloud.google.com/storage/docs/resumable-uploads#go for details.
const gcsMinUploadChunkSize = 256 * 1024 // 256 KiB

// Validate checks if the GcpStorage is valid.
func (s *GcpStorage) Validate(opts ...ValidationOption) error {
	if s.BucketName == "" {
		return errors.New("GCP bucket name is not specified")
	}
	if err := validateObjectStoragePath(s.Path); err != nil {
		return err
	}
	if s.KeyFile != "" && s.Key != "" {
		return errValidationMutuallyExclusive("key-file-path", "key-json")
	}
	if err := s.StorageClass.Validate(); err != nil {
		return fmt.Errorf("invalid storage class: %w", err)
	}
	if s.MinPartSize != nil && *s.MinPartSize < gcsMinUploadChunkSize {
		return errValidationInvalidValue("min-part-size", *s.MinPartSize, "at least 256KiB")
	}

	withAgent := s.hasSecretAgent()
	if err := s.Key.Validate(withAgent); err != nil {
		return errValidationSecret("key-json", err)
	}

	//nolint:staticcheck // We want to call embedded methods with embedded struct name.
	return s.SecretAgentConfig.validate(opts...)
}

func (s *GcpStorage) toModel(config *model.Config) (model.Storage, error) {
	//nolint:staticcheck // We want to call embedded methods with embedded struct name.
	agent, err := s.SecretAgentConfig.ToModel(config)
	if err != nil {
		return nil, err
	}

	return &model.GcpStorage{
		KeyFile:      s.KeyFile,
		BucketName:   s.BucketName,
		Path:         s.Path,
		Endpoint:     s.Endpoint,
		KeyJSON:      string(s.Key),
		SecretAgent:  agent,
		MinPartSize:  s.MinPartSize,
		StorageClass: s.StorageClass.ToModel(),
	}, nil
}

func newGcpStorageFromModel(s *model.GcpStorage, config *model.BackupConfig) *GcpStorage {
	return &GcpStorage{
		KeyFile:           s.KeyFile,
		BucketName:        s.BucketName,
		Path:              s.Path,
		Endpoint:          s.Endpoint,
		Key:               secret(s.KeyJSON),
		MinPartSize:       s.MinPartSize,
		SecretAgentConfig: ResolveSecretAgentFromModel(s.SecretAgent, config),
		StorageClass:      newGcpStorageClassFromModel(s.StorageClass),
	}
}

// GCP Storage Classes
// See https://cloud.google.com/storage/docs/storage-classes
const (
	GcpClassStandard = "STANDARD"
	GcpClassNearline = "NEARLINE"
	GcpClassColdline = "COLDLINE"
	GcpClassArchive  = "ARCHIVE"
)

// data can be stored in any class.
var gcpDataClasses = []string{
	"",
	GcpClassStandard,
	GcpClassNearline,
	GcpClassColdline,
	GcpClassArchive,
}

// GcpStorageClass represents the configuration for GCP Storage Class.
// @Description GcpStorageClass represents the configuration for GCP Storage Class.
type GcpStorageClass struct {
	// DataClass specifies the storage class for object data.
	DataClass string `json:"data" yaml:"data" extensions:"x-nullable" enums:"STANDARD,NEARLINE,COLDLINE,ARCHIVE"`
}

func (s *GcpStorageClass) Validate() error {
	if s == nil {
		return nil
	}

	if !slices.Contains(gcpDataClasses, s.DataClass) {
		return errValidationInvalidValue("data", s.DataClass, gcpDataClasses)
	}

	return nil
}

func (s *GcpStorageClass) ToModel() *model.StorageClass {
	if s == nil {
		return nil
	}

	return &model.StorageClass{
		DataClass:     s.DataClass,
		MetadataClass: "", // GCP metadata always uses STANDARD class because it's the only one without retrieval fees.
	}
}

func newGcpStorageClassFromModel(s *model.StorageClass) *GcpStorageClass {
	if s == nil {
		return nil
	}

	return &GcpStorageClass{
		DataClass: s.DataClass,
	}
}
