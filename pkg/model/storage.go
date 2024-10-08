package model

import "fmt"

// Storage represents the configuration for a backup storage details.
// This interface is implemented by all specific storage types.
type Storage interface {
	storage()
}

type LocalStorage struct {
	// Path is the root directory where backups will be stored locally.
	Path string
}

func (s *LocalStorage) storage() {}
func (s *LocalStorage) String() string {
	return fmt.Sprintf("LocalStorage(Path: %s)", s.Path)
}

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
}

func (s *S3Storage) storage() {}
func (s *S3Storage) String() string {
	return fmt.Sprintf("S3Storage(Bucket: %s, Path: %s)", s.Bucket, s.Path)
}

type GcpStorage struct {
	// KeyFile is the path to the JSON file containing the Google Cloud service account key.
	// This file is used for authentication with GCP services.
	KeyFile string
	// BucketName is the name of the GCP bucket where backups will be stored.
	BucketName string
	// Path is the root directory within the GCS bucket where backups will be stored.
	Path string
	// Endpoint is an alternative URL for the GCS API.
	// This should only be used for testing or in specific non-production scenarios.
	Endpoint string
}

func (s *GcpStorage) storage() {}
func (s *GcpStorage) String() string {
	return fmt.Sprintf("GcpStorage(Bucket: %s, Path: %s)", s.BucketName, s.Path)
}

// AzureStorage represents the configuration for Azure Blob storage.
type AzureStorage struct {
	// Path is the root directory within the Azure Blob container where backups will be stored.
	Path string
	// Endpoint is the URL of the Azure Blob storage service.
	Endpoint string
	// ContainerName is the name of the Azure Blob container where backups will be stored.
	ContainerName string
	// Auth holds the authentication details for Azure Blob storage.
	// It can be either AzureSharedKeyAuth or AzureADAuth.
	Auth AzureAuth
}

func (s *AzureStorage) storage() {}

// AzureAuth represents the authentication methods for Azure Blob storage.
// This interface is implemented by AzureSharedKeyAuth and AzureADAuth.
type AzureAuth interface {
	azureAuth()
}

// AzureSharedKeyAuth represents shared key authentication for Azure Blob storage.
type AzureSharedKeyAuth struct {
	// AccountName is the name of the Azure Storage account.
	AccountName string
	// AccountKey is the access key for the Azure Storage account.
	AccountKey string
}

func (AzureSharedKeyAuth) azureAuth() {}

// AzureADAuth represents Azure Active Directory authentication for Azure Blob storage.
type AzureADAuth struct {
	// TenantID is the Azure AD tenant (directory) ID.
	TenantID string
	// ClientID is the application (client) ID registered in Azure AD.
	ClientID string
	// ClientSecret is the secret key for the application registered in Azure AD.
	ClientSecret string
}

func (AzureADAuth) azureAuth() {}
