package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
	azure "github.com/aerospike/backup-go/io/storage/azure/blob"
	"github.com/aerospike/backup-go/io/storage/options"
)

type AzureStorageAccessor struct {
	clientMap collections.CacheContext[*model.AzureStorage, *azblob.Client]
	resolver  secrets.Resolver
}

func NewAzureStorageAccessor(ctx context.Context, resolver secrets.Resolver) *AzureStorageAccessor {
	accessor := &AzureStorageAccessor{
		resolver: resolver,
	}
	accessor.clientMap = collections.NewLoadingCacheContext[*model.AzureStorage, *azblob.Client](
		ctx,
		accessor.getAzureClient,
		ptr.Of(time.Hour),
	)
	return accessor
}

func (a *AzureStorageAccessor) supports(storage model.Storage) bool {
	_, ok := storage.(*model.AzureStorage)
	return ok
}

func (a *AzureStorageAccessor) createReader(ctx context.Context, storage model.Storage, opts ...options.Opt,
) (backup.StreamingReader, error) {
	azures := storage.(*model.AzureStorage)
	client, err := a.clientMap.Get(ctx, azures)
	if err != nil {
		return nil, err
	}

	return azure.NewReader(ctx, client, azures.ContainerName, opts...)
}

func (a *AzureStorageAccessor) createWriter(
	ctx context.Context, storage model.Storage, opts ...options.Opt,
) (backup.Writer, error) {
	azures := storage.(*model.AzureStorage)
	client, err := a.clientMap.Get(ctx, azures)
	if err != nil {
		return nil, err
	}

	if azures.MinPartSize != nil {
		opts = append(opts, options.WithChunkSize(*azures.MinPartSize))
	}

	return azure.NewWriter(ctx, client, azures.ContainerName, opts...)
}

func (a *AzureStorageAccessor) getAzureClient(ctx context.Context, s *model.AzureStorage) (*azblob.Client, error) {
	client, err := a.createAzureClient(s)

	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob client: %w", err)
	}

	if err := checkAzureConnectivity(ctx, client, s.ContainerName); err != nil {
		return nil, err
	}

	return client, nil
}

func (a *AzureStorageAccessor) createAzureClient(s *model.AzureStorage) (*azblob.Client, error) {
	switch auth := s.Auth.(type) {
	case *model.AzureSharedKeyAuth:
		return a.clientFromSharedKey(s.Endpoint, auth, s.SecretAgent)
	case *model.AzureADAuth:
		return a.clientFromAD(s.Endpoint, auth, s.SecretAgent)
	default:
		return clientWithDefaultCredential(s.Endpoint)
	}
}

func (a *AzureStorageAccessor) clientFromSharedKey(
	endpoint string, auth *model.AzureSharedKeyAuth, sa *model.SecretAgent,
) (*azblob.Client, error) {
	accountKey, err := a.resolver.Resolve(sa, auth.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account key from secret agent: %w", err)
	}
	cred, err := azblob.NewSharedKeyCredential(auth.AccountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure shared key credentials: %w", err)
	}

	client, err := azblob.NewClientWithSharedKeyCredential(endpoint, cred, azureOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob client with shared key: %w", err)
	}

	return client, nil
}

func (a *AzureStorageAccessor) clientFromAD(
	endpoint string,
	auth *model.AzureADAuth,
	sa *model.SecretAgent,
) (*azblob.Client, error) {
	clientSecret, err := a.resolver.Resolve(sa, auth.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve client-secret from secret agent: %w", err)
	}
	tenantID, err := a.resolver.Resolve(sa, auth.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tenant-id from secret agent: %w", err)
	}
	clientID, err := a.resolver.Resolve(sa, auth.ClientID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve client-id from secret agent: %w", err)
	}
	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure AAD credentials: %w", err)
	}

	client, err := azblob.NewClient(endpoint, cred, azureOptions())
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

	client, err := azblob.NewClient(endpoint, cred, azureOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob client with default credentials: %w", err)
	}

	return client, nil
}

func azureOptions() *azblob.ClientOptions {
	return &azblob.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Retry: policy.RetryOptions{
				MaxRetries:    int32(model.StorageRetryPolicy.MaxRetries),
				RetryDelay:    model.StorageRetryPolicy.BaseTimeout,
				MaxRetryDelay: model.StorageRetryPolicy.MaxBackoffDuration,
			},
		},
	}
}

func checkAzureConnectivity(ctx context.Context, client *azblob.Client, container string) error {
	ctx, cancel := context.WithTimeout(ctx, connectivityTimeout)
	defer cancel()

	cc := client.ServiceClient().NewContainerClient(container)
	_, err := cc.GetProperties(ctx, nil)
	if err != nil {
		return fmt.Errorf("azure blob storage connectivity check failed: %w", err)
	}

	return nil
}
