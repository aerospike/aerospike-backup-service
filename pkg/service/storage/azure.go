package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/backup-go"
	azure "github.com/aerospike/backup-go/io/storage/azure/blob"
	"github.com/aerospike/backup-go/io/storage/options"
)

type AzureStorageAccessor struct {
	clientMap *collections.LoadingCache[*model.AzureStorage, *azblob.Client]
}

func NewAzureStorageAccessor() *AzureStorageAccessor {
	return &AzureStorageAccessor{
		clientMap: collections.NewLoadingCache[*model.AzureStorage, *azblob.Client](context.Background(), getAzureClient),
	}
}

func (a *AzureStorageAccessor) supports(storage model.Storage) bool {
	_, ok := storage.(*model.AzureStorage)
	return ok
}

func (a *AzureStorageAccessor) createReader(ctx context.Context, storage model.Storage, opts ...options.Opt,
) (backup.StreamingReader, error) {
	azures := storage.(*model.AzureStorage)
	client, err := a.clientMap.Get(azures)
	if err != nil {
		return nil, err
	}

	return azure.NewReader(ctx, client, azures.ContainerName, opts...)
}

func (a *AzureStorageAccessor) createWriter(
	ctx context.Context, storage model.Storage, opts ...options.Opt,
) (backup.Writer, error) {
	azures := storage.(*model.AzureStorage)
	client, err := a.clientMap.Get(azures)
	if err != nil {
		return nil, err
	}

	if azures.MinPartSize != nil {
		opts = append(opts, options.WithChunkSize(*azures.MinPartSize))
	}

	return azure.NewWriter(ctx, client, azures.ContainerName, opts...)
}

func init() {
	registerAccessor(NewAzureStorageAccessor())
}

func getAzureClient(ctx context.Context, a *model.AzureStorage) (*azblob.Client, error) {
	client, err := createAzureClient(a)

	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob client: %w", err)
	}

	if err := checkAzureConnectivity(ctx, client, a.ContainerName); err != nil {
		return nil, err
	}

	return client, nil
}

func createAzureClient(a *model.AzureStorage) (*azblob.Client, error) {
	switch auth := a.Auth.(type) {
	case *model.AzureSharedKeyAuth:
		return clientFromSharedKey(a.Endpoint, auth, a.SecretAgent)
	case *model.AzureADAuth:
		return clientFromAD(a.Endpoint, auth, a.SecretAgent)
	default:
		slog.Info("Using default Azure credentials (unknown type)",
			slog.Any("auth_type", fmt.Sprintf("%T", auth)),
			slog.Any("auth", auth))
		return clientWithDefaultCredential(a.Endpoint)
	}
}

func clientFromSharedKey(
	endpoint string, auth *model.AzureSharedKeyAuth, sa *model.SecretAgent,
) (*azblob.Client, error) {
	slog.Info("Using Shared key Azure credentials", slog.Any("auth", auth))
	accountKey, err := sa.Read(auth.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account key from secret agent: %w", err)
	}
	cred, err := azblob.NewSharedKeyCredential(auth.AccountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure shared key credentials: %w", err)
	}

	client, err := azblob.NewClientWithSharedKeyCredential(endpoint, cred, &azblob.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Retry: policy.RetryOptions{
				MaxRetries:    int32(model.StorageRetryPolicy.MaxRetries),
				RetryDelay:    model.StorageRetryPolicy.BaseTimeout,
				MaxRetryDelay: model.StorageRetryPolicy.MaxBackoffDuration,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob client with shared key: %w", err)
	}

	return client, nil
}

func clientFromAD(endpoint string, auth *model.AzureADAuth, sa *model.SecretAgent) (*azblob.Client, error) {
	slog.Info("Using AD Azure credentials", slog.Any("auth", auth))
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

	client, err := azblob.NewClient(endpoint, cred, &azblob.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Retry: policy.RetryOptions{
				MaxRetries:    int32(model.StorageRetryPolicy.MaxRetries),
				RetryDelay:    model.StorageRetryPolicy.BaseTimeout,
				MaxRetryDelay: model.StorageRetryPolicy.MaxBackoffDuration,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob client with AAD: %w", err)
	}

	return client, nil
}

func clientWithDefaultCredential(endpoint string) (*azblob.Client, error) {
	slog.Info("Using default Azure credentials", slog.String("endpoint", endpoint))
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain default Azure credentials: %w", err)
	}

	client, err := azblob.NewClient(endpoint, cred, &azblob.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Retry: policy.RetryOptions{
				MaxRetries:    int32(model.StorageRetryPolicy.MaxRetries),
				RetryDelay:    model.StorageRetryPolicy.BaseTimeout,
				MaxRetryDelay: model.StorageRetryPolicy.MaxBackoffDuration,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob client with default credentials: %w", err)
	}

	return client, nil
}

func checkAzureConnectivity(ctx context.Context, client *azblob.Client, container string) error {
	ctx, cancel := context.WithTimeout(ctx, connectivityTimeout)
	defer cancel()

	cc := client.ServiceClient().NewContainerClient(container)
	_, err := cc.GetProperties(ctx, nil)
	if err != nil {
		return fmt.Errorf("azure blob storage connectivity check failed: %w", err)
	}

	slog.Info("Azure connectivity check succeeded")

	return nil
}
