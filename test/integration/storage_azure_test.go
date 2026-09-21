//go:build integration

package integration

import (
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	azuriteImage    = "mcr.microsoft.com/azure-storage/azurite:3.37.0"
	azuriteBlobPort = "10000/tcp"

	// azuriteAccountName/Key are Azurite's well-known default development account:
	// https://learn.microsoft.com/en-us/azure/storage/common/storage-use-azurite#well-known-storage-account-and-key
	// Not a real secret - fixed by Azurite itself, the same for every user.
	azuriteAccountName = "devstoreaccount1"
	//nolint:gosec // Azurite's fixed public dev-account key, not a real secret
	azuriteAccountKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

	azureContainerName = "abs-integration-test"

	azureAccountKeySecretName = "azure-account-key"
)

// TestAzure exercises every way ABS can authenticate to an Azure Blob endpoint that is
// reachable against a self-hosted emulator, against one shared Azurite container: Shared
// Key (literal and Secret Agent) and a SAS token embedded in the endpoint URL.
//
// AAD (service-principal) and Managed Identity auth (clientFromAD / clientWithDefaultCredential
// in pkg/service/storage/azure.go) are not covered here. Both call the real Azure AD token
// endpoint (login.microsoftonline.com) with no way to point them at a local stand-in: ABS
// passes `nil` options to azidentity.NewClientSecretCredential/NewDefaultAzureCredential, so
// there is no authority-host override to redirect at Azurite even though Azurite's own
// `--oauth basic` mode would accept whatever bearer token showed up. Making AAD
// emulator-testable would mean adding a customizable authority host to ABS's Azure storage
// config - a real product change, not just test infrastructure - so it is left as a
// documented gap rather than done here. AAD/Managed Identity remain real-Azure-only.
func (s *StorageSuite) TestAzure() {
	endpoint, admin := s.startAzurite()

	s.Run("shared key, literal values", func() {
		s.storageBackup(&dto.Storage{AzureStorage: &dto.AzureStorage{
			Endpoint:      endpoint,
			ContainerName: azureContainerName,
			AccountName:   azuriteAccountName,
			AccountKey:    azuriteAccountKey,
		}})
	})

	s.Run("shared key, account-key from secret agent", func() {
		agent := s.startSecretAgentWithKeys(map[string]string{
			azureAccountKeySecretName: azuriteAccountKey,
		})

		s.storageBackup(&dto.Storage{AzureStorage: &dto.AzureStorage{
			SecretAgentConfig: dto.SecretAgentConfig{SecretAgent: agent},
			Endpoint:          endpoint,
			ContainerName:     azureContainerName,
			AccountName:       azuriteAccountName,
			AccountKey:        redact.Secret(secretRefKey(azureAccountKeySecretName)),
		}})
	})

	s.Run("SAS token embedded in endpoint", func() {
		sasEndpoint := s.accountSAS(admin)

		s.storageBackup(&dto.Storage{AzureStorage: &dto.AzureStorage{
			Endpoint:      sasEndpoint,
			ContainerName: azureContainerName,
		}})
	})
}

// startAzurite starts an Azurite container and creates the container the Azure tests
// write to. It returns the service endpoint (dto.AzureStorage.Endpoint for Shared Key
// auth) and the admin client used to create it, which sub-tests reuse to mint SAS tokens.
func (s *StorageSuite) startAzurite() (string, *azblob.Client) {
	ctx := s.T().Context()

	container, err := testcontainers.Run(ctx, azuriteImage,
		testcontainers.WithExposedPorts(azuriteBlobPort),
		testcontainers.WithWaitStrategy(wait.ForListeningPort(azuriteBlobPort)),
	)
	s.Require().NoError(err)
	s.cleanupContainer(container)

	host, err := container.Host(ctx)
	s.Require().NoError(err)

	mapped, err := container.MappedPort(ctx, azuriteBlobPort)
	s.Require().NoError(err)

	endpoint := fmt.Sprintf("http://%s:%d/%s", host, mapped.Num(), azuriteAccountName)

	cred, err := azblob.NewSharedKeyCredential(azuriteAccountName, azuriteAccountKey)
	s.Require().NoError(err)
	admin, err := azblob.NewClientWithSharedKeyCredential(endpoint, cred, nil)
	s.Require().NoError(err)

	_, err = admin.CreateContainer(ctx, azureContainerName, nil)
	s.Require().NoError(err)

	return endpoint, admin
}

// accountSAS mints a short-lived account-level SAS token (full read/write/list/create/
// delete, scoped to containers and objects) and returns it appended to the service
// endpoint - exactly the "endpoint with an embedded SAS" shape
// pkg/service/storage/azure.go's endpointHasEmbeddedSAS looks for.
func (s *StorageSuite) accountSAS(admin *azblob.Client) string {
	sasURL, err := admin.ServiceClient().GetSASURL(
		sas.AccountResourceTypes{Service: true, Container: true, Object: true},
		sas.AccountPermissions{Read: true, Write: true, Create: true, List: true, Delete: true, Add: true},
		time.Now().Add(time.Hour),
		nil,
	)
	s.Require().NoError(err)

	return sasURL
}
