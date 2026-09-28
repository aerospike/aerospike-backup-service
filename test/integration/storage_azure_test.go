//go:build integration

package integration

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
)

// TestAzure exercises every way ABS can authenticate to an Azure Blob endpoint that is
// reachable against a self-hosted emulator, against one shared azuriteServer container: Shared
// Key (literal and Secret Agent) and a SAS token embedded in the endpoint URL. AAD and
// Managed Identity are not covered: they cannot be pointed at an emulator.
//
// AAD (service-principal) and Managed Identity auth (clientFromAD / clientWithDefaultCredential
// in pkg/service/storage/azure.go) are not covered here. Both call the real Azure AD token
// endpoint (login.microsoftonline.com) with no way to point them at a local stand-in: ABS
// passes `nil` options to azidentity.NewClientSecretCredential/NewDefaultAzureCredential, so
// there is no authority-host override to redirect at azuriteServer even though azuriteServer's own
// `--oauth basic` mode would accept whatever bearer token showed up. Making AAD
// emulator-testable would mean adding a customizable authority host to ABS's Azure storage
// config - a real product change, not just test infrastructure - so it is left as a
// documented gap rather than done here. AAD/Managed Identity remain real-Azure-only.
func (s *StorageSuite) TestAzure() {
	azurite := s.startAzurite()

	s.Run("shared key, literal values", func() {
		s.assertBackupToStorage(&dto.Storage{AzureStorage: &dto.AzureStorage{
			Endpoint:      azurite.Endpoint,
			ContainerName: azureContainerName,
			AccountName:   azuriteAccountName,
			AccountKey:    azuriteAccountKey,
		}})
	})

	s.Run("shared key, account-key from secret agent", func() {
		agent := s.startSecretAgent(azuriteAccountKey)

		s.assertBackupToStorage(&dto.Storage{AzureStorage: &dto.AzureStorage{
			SecretAgentConfig: dto.SecretAgentConfig{SecretAgent: agent},
			Endpoint:          azurite.Endpoint,
			ContainerName:     azureContainerName,
			AccountName:       azuriteAccountName,
			AccountKey:        redact.Secret(secretRef()),
		}})
	})

	s.Run("SAS token embedded in endpoint", func() {
		s.assertBackupToStorage(&dto.Storage{AzureStorage: &dto.AzureStorage{
			Endpoint:      s.accountSASEndpoint(azurite),
			ContainerName: azureContainerName,
		}})
	})
}
