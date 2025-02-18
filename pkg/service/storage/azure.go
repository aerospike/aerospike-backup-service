package storage

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	ioStorage "github.com/aerospike/backup-go/io/storage"
	azure "github.com/aerospike/backup-go/io/storage/azure/blob"
)

type AzureStorageAccessor struct{}

func (a *AzureStorageAccessor) supports(storage model.Storage) bool {
	_, ok := storage.(*model.AzureStorage)
	return ok
}

func (a *AzureStorageAccessor) createReader(ctx context.Context, storage model.Storage, opts ...ioStorage.Opt,
) (backup.StreamingReader, error) {
	azures := storage.(*model.AzureStorage)
	client, err := getAzureClient(azures)
	if err != nil {
		return nil, err
	}

	return azure.NewReader(ctx, client, azures.ContainerName, opts...)
}

func (a *AzureStorageAccessor) createWriter(
	ctx context.Context, storage model.Storage, opts ...ioStorage.Opt,
) (backup.Writer, error) {
	azures := storage.(*model.AzureStorage)
	client, err := getAzureClient(azures)
	if err != nil {
		return nil, err
	}

	return azure.NewWriter(ctx, client, azures.ContainerName, opts...)
}

func init() {
	registerAccessor(&AzureStorageAccessor{})
}

func getAzureClient(a *model.AzureStorage) (*azblob.Client, error) {
	switch auth := a.Auth.(type) {
	case model.AzureSharedKeyAuth:
		return clientFromSharedKey(a.Endpoint, auth, a.SecretAgent)
	case model.AzureADAuth:
		return clientFromAD(a.Endpoint, auth, a.SecretAgent)
	default:
		return clientWithDefaultCredential(a.Endpoint)
	}
}

func clientFromSharedKey(
	endpoint string, auth model.AzureSharedKeyAuth, sa *model.SecretAgent,
) (*azblob.Client, error) {
	accountKey, err := sa.Read(auth.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account key from secret agent: %w", err)
	}
	cred, err := azblob.NewSharedKeyCredential(auth.AccountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure shared key credentials: %w", err)
	}

	client, err := azblob.NewClientWithSharedKeyCredential(endpoint, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob client with shared key: %w", err)
	}

	return client, nil
}

func clientFromAD(endpoint string, auth model.AzureADAuth, sa *model.SecretAgent) (*azblob.Client, error) {
	clientSecret, err := sa.Read(auth.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve client-secret from secret agent: %w", err)
	}
	tenantID, err := sa.Read(auth.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tenant-id from secret agent: %w", err)
	}
	clientID, err := sa.Read(auth.ClientID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve client-id from secret agent: %w", err)
	}
	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure AAD credentials: %w", err)
	}

	client, err := azblob.NewClient(endpoint, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob client with AAD: %w", err)
	}

	return client, nil
}

func clientWithDefaultCredential(endpoint string) (*azblob.Client, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain default Azure credentials: %w", err)
	}

	client, err := azblob.NewClient(endpoint, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob client with default credentials: %w", err)
	}

	return client, nil
}
