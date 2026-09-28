//go:build integration

package integration

import (
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The azuriteServer account and the container startAzurite creates.
//
// The account is azuriteServer's well-known development account, the same for every user:
// https://learn.microsoft.com/en-us/azure/storage/common/storage-use-azurite#well-known-storage-account-and-key
const (
	azuriteAccountName = "devstoreaccount1"
	azuriteAccountKey  = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

	azureContainerName = "abs-integration-test"
)

const (
	azuriteImage    = "mcr.microsoft.com/azure-storage/azurite:3.37.0"
	azuriteBlobPort = "10000/tcp"
)

// azuriteServer is a running Azure Blob emulator.
type azuriteServer struct {
	// Endpoint is the service URL ABS reaches it at (dto.AzureStorage.Endpoint).
	Endpoint string

	admin *azblob.Client
}

// startAzurite starts an azuriteServer container and creates azureContainerName in it.
func (s *Suite) startAzurite() azuriteServer {
	ctx := s.T().Context()

	azurite, err := testcontainers.Run(ctx, azuriteImage,
		testcontainers.WithExposedPorts(azuriteBlobPort),
		testcontainers.WithWaitStrategy(wait.ForListeningPort(azuriteBlobPort)),
	)
	s.Require().NoError(err)
	s.terminateOnCleanup(azurite, "azurite")

	host, err := azurite.Host(ctx)
	s.Require().NoError(err)

	mapped, err := azurite.MappedPort(ctx, azuriteBlobPort)
	s.Require().NoError(err)

	endpoint := fmt.Sprintf("http://%s:%d/%s", host, mapped.Num(), azuriteAccountName)

	cred, err := azblob.NewSharedKeyCredential(azuriteAccountName, azuriteAccountKey)
	s.Require().NoError(err)
	admin, err := azblob.NewClientWithSharedKeyCredential(endpoint, cred, nil)
	s.Require().NoError(err)

	_, err = admin.CreateContainer(ctx, azureContainerName, nil)
	s.Require().NoError(err)

	return azuriteServer{Endpoint: endpoint, admin: admin}
}

// accountSASEndpoint mints a one-hour account-level SAS token (read, write, list,
// create, delete; containers and objects) and returns it appended to the service
// endpoint: the "endpoint with an embedded SAS" shape
// pkg/service/storage/azure.go's endpointHasEmbeddedSAS looks for.
func (s *Suite) accountSASEndpoint(azurite azuriteServer) string {
	sasURL, err := azurite.admin.ServiceClient().GetSASURL(
		sas.AccountResourceTypes{Service: true, Container: true, Object: true},
		sas.AccountPermissions{Read: true, Write: true, Create: true, List: true, Delete: true, Add: true},
		time.Now().Add(time.Hour),
		nil,
	)
	s.Require().NoError(err)

	return sasURL
}
