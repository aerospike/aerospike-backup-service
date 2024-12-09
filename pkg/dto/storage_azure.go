package dto

import (
	"errors"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// AzureStorage represents the configuration for Azure Blob storage.
type AzureStorage struct {
	SecretAgentConfig `yaml:",inline"`
	// Endpoint is the Azure Blob service endpoint URL.
	Endpoint string `yaml:"endpoint" json:"endpoint" validate:"required"`
	// ContainerName is the name of the Azure Blob container.
	ContainerName string `yaml:"container-name" json:"container-name" validate:"required"`
	// Path is the root path for the backup repository within the container.
	// If not specified, backups will be saved in the container's root.
	Path string `yaml:"path,omitempty" json:"path,omitempty" example:"backups"`
	// AccountName is the Azure storage account name for Shared Key authentication.
	AccountName string `yaml:"account-name,omitempty" json:"account-name,omitempty"`
	// AccountKey is the Azure storage account key for Shared Key authentication.
	// This is sensitive information. Can be a path in secret agent or an actual value.
	AccountKey string `yaml:"account-key,omitempty" json:"account-key,omitempty"`
	// TenantID is the Azure Active Directory tenant ID for AAD authentication.
	TenantID string `yaml:"tenant-id,omitempty" json:"tenant-id,omitempty"`
	// ClientID is the Azure Active Directory client ID for AAD authentication.
	ClientID string `yaml:"client-id,omitempty" json:"client-id,omitempty"`
	// ClientSecret is the Azure Active Directory client secret for AAD authentication.
	// This is sensitive information. Can be a path in secret agent or an actual value.
	ClientSecret string `yaml:"client-secret,omitempty" json:"client-secret,omitempty"`
}

// Validate checks if the AzureStorage is valid.
func (a *AzureStorage) Validate() error {
	if a.Endpoint == "" {
		return errors.New("azure storage endpoint is not specified")
	}
	if a.ContainerName == "" {
		return errors.New("azure storage container name is not specified")
	}

	// Check for valid authentication method.
	hasSharedKey := a.AccountName != "" && a.AccountKey != ""
	hasAAD := a.TenantID != "" && a.ClientID != "" && a.ClientSecret != ""

	if hasSharedKey && hasAAD {
		return errors.New(`azure storage authentication method is ambiguous:
use either AccountName/AccountKey or TenantID/ClientID/ClientSecret, not both`)
	}

	return nil
}

func (a *AzureStorage) toModel(c *model.Config) (model.Storage, error) {
	agent, err := c.ResolveSecretAgent(a.SecretAgentName, a.SecretAgent.ToModel())
	if err != nil {
		return nil, err
	}
	return &model.AzureStorage{
		Endpoint:      a.Endpoint,
		ContainerName: a.ContainerName,
		Path:          a.Path,
		Auth:          getAzureAuth(a),
		SecretAgent:   agent,
	}, nil
}

func getAzureAuth(a *AzureStorage) model.AzureAuth {
	if a.AccountName != "" && a.AccountKey != "" {
		return model.AzureSharedKeyAuth{
			AccountName: a.AccountName,
			AccountKey:  a.AccountKey,
		}
	}

	if a.TenantID != "" && a.ClientID != "" && a.ClientSecret != "" {
		return model.AzureADAuth{
			TenantID:     a.TenantID,
			ClientID:     a.ClientID,
			ClientSecret: a.ClientSecret,
		}
	}

	return nil
}

func newAzureStorageFromModel(s *model.AzureStorage, config *model.Config) *AzureStorage {
	azureStorage := &AzureStorage{
		Endpoint:          s.Endpoint,
		ContainerName:     s.ContainerName,
		Path:              s.Path,
		SecretAgentConfig: ResolveSecretAgentFromModel(s.SecretAgent, config),
	}

	switch auth := s.Auth.(type) {
	case model.AzureSharedKeyAuth:
		azureStorage.AccountName = auth.AccountName
		azureStorage.AccountKey = auth.AccountKey
	case model.AzureADAuth:
		azureStorage.TenantID = auth.TenantID
		azureStorage.ClientID = auth.ClientID
		azureStorage.ClientSecret = auth.ClientSecret
	}

	return azureStorage
}
