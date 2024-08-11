package dto

import "github.com/aerospike/backup/pkg/model"

// StorageDTO represents the configuration for a backup storage details.
// @Description StorageDTO represents the configuration for a backup storage details.
type StorageDTO struct {
	// The type of the storage provider
	Type string `json:"type" enums:"local,aws-s3"`
	// The root path for the backup repository.
	Path *string `json:"path,omitempty" example:"backups"`
	// The S3 region string (AWS S3 optional).
	S3Region *string `json:"s3-region,omitempty" example:"eu-central-1"`
	// The S3 profile name (AWS S3 optional).
	S3Profile *string `json:"s3-profile,omitempty" example:"default"`
	// An alternative endpoint for the S3 SDK to communicate (AWS S3 optional).
	S3EndpointOverride *string `yjson:"s3-endpoint-override,omitempty" example:"http://host.docker.internal:9000"`
	// The log level of the AWS S3 SDK (AWS S3 optional).
	S3LogLevel *string `json:"s3-log-level,omitempty" default:"FATAL" enum:"OFF,FATAL,ERROR,WARN,INFO,DEBUG,TRACE"`
	// The minimum size in bytes of individual S3 UploadParts
	MinPartSize int `json:"min_part_size,omitempty" example:"10" default:"5242880"`
	// The maximum number of simultaneous requests from S3.
	MaxConnsPerHost int `json:"max_async_connections,omitempty" example:"16"`
}

// MapStorageFromDTO maps StorageDTO to model.Storage
func MapStorageFromDTO(dto StorageDTO) model.Storage {
	return model.Storage{
		Type:               model.StorageType(dto.Type),
		Path:               dto.Path,
		S3Region:           dto.S3Region,
		S3Profile:          dto.S3Profile,
		S3EndpointOverride: dto.S3EndpointOverride,
		S3LogLevel:         dto.S3LogLevel,
		MinPartSize:        dto.MinPartSize,
		MaxConnsPerHost:    dto.MaxConnsPerHost,
	}
}
