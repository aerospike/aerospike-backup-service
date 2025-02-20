package model

import "fmt"

// AzureStorage represents the configuration for Azure Blob storage.
type AzureStorage struct {
	// Path is the root directory within the Azure Blob container where backups will be stored.
	Path string
	// Endpoint is the URL of the Azure Blob storage service.
	Endpoint string
	// ContainerName is the name of the Azure Blob container where backups will be stored.
	ContainerName string
	// Auth holds the authentication details for Azure Blob storage.
	// It can be nil or AzureSharedKeyAuth or AzureADAuth.
	Auth AzureAuth
	// SecretAgent configuration to fetch keyfile from a secret store (optional).
	SecretAgent *SecretAgent
}

func (s *AzureStorage) GetStorageClass() StorageClass {
	return StorageClass{}
}

func (s *AzureStorage) GetPath() string {
	return s.Path
}

func (s *AzureStorage) String() string {
	return fmt.Sprintf("AzureStorage(Endpoint: %s, Container: %s, Path: %s)", s.Endpoint, s.ContainerName, s.Path)
}

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
