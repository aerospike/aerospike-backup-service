package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
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
	client, err := a.createAzureClient(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob client: %w", err)
	}

	if err := checkAzureConnectivity(ctx, client, s.ContainerName); err != nil {
		return nil, err
	}

	return client, nil
}

func (a *AzureStorageAccessor) createAzureClient(ctx context.Context, s *model.AzureStorage) (*azblob.Client, error) {
	switch auth := s.Auth.(type) {
	case *model.AzureSharedKeyAuth:
		return a.clientFromSharedKey(ctx, s.Endpoint, auth, s.SecretAgent)
	case *model.AzureADAuth:
		return a.clientFromAD(ctx, s.Endpoint, auth, s.SecretAgent)
	default:
		return a.noAuthClient(s)
	}
}

func (a *AzureStorageAccessor) clientFromSharedKey(
	ctx context.Context,
	endpoint string,
	auth *model.AzureSharedKeyAuth,
	sa *model.SecretAgent,
) (*azblob.Client, error) {
	accountKey, err := a.resolver.Resolve(ctx, sa, auth.AccountKey)
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
	ctx context.Context,
	endpoint string,
	auth *model.AzureADAuth,
	sa *model.SecretAgent,
) (*azblob.Client, error) {
	clientSecret, err := a.resolver.Resolve(ctx, sa, auth.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve client-secret from secret agent: %w", err)
	}
	tenantID, err := a.resolver.Resolve(ctx, sa, auth.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tenant-id from secret agent: %w", err)
	}
	clientID, err := a.resolver.Resolve(ctx, sa, auth.ClientID)
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

func (a *AzureStorageAccessor) noAuthClient(s *model.AzureStorage) (*azblob.Client, error) {
	isSas, err := endpointHasEmbeddedSAS(s.Endpoint)
	if err != nil {
		return nil, err
	}

	if isSas {
		client, err := azblob.NewClientWithNoCredential(s.Endpoint, azureOptions())
		if err != nil {
			return nil, fmt.Errorf("failed to create Azure Blob client with SAS URL: %w", err)
		}
		return client, nil
	}

	return clientWithDefaultCredential(s.Endpoint)
}

func endpointHasEmbeddedSAS(endpoint string) (bool, error) {
	parts, err := sas.ParseURL(endpoint)
	if err != nil {
		return false, fmt.Errorf("failed to parse URL: %w", err)
	}

	return parts.SAS.Signature() != "", nil
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

	_, err = cc.NewListBlobsFlatPager(&azblob.ListBlobsFlatOptions{
		MaxResults: ptr.Of(int32(1)),
	}).NextPage(ctx)
	if err != nil {
		return fmt.Errorf("azure blob storage read permission check failed: %w", err)
	}

	blob := cc.NewBlockBlobClient(connectivityProbeKey)
	_, err = blob.UploadBuffer(ctx, []byte{}, nil)
	if err != nil {
		slog.Warn("azure blob storage upload permission check failed; backup writes may fail at runtime",
			slog.String("container", container),
			attr.Error(err),
		)
		return nil
	}

	_, err = blob.Delete(ctx, nil)
	if err != nil && !bloberror.HasCode(err, bloberror.BlobNotFound) {
		slog.Warn("azure blob storage delete permission check failed; backup writes or cleanup may fail at runtime",
			slog.String("container", container),
			attr.Error(err),
		)
	}

	return nil
}
