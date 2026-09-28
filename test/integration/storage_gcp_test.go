//go:build integration

package integration

import (
	"os"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
)

// TestGCP exercises every way ABS can authenticate to a GCS-compatible endpoint, against
// one shared fake-gcs-server container: the no-auth Endpoint-only path, and real
// service-account key auth (both key-file and key-JSON, literal and Secret Agent) with
// the credential exchange redirected at a local fake OAuth token endpoint instead of
// Google's real one. ADC (Application Default Credentials, the path taken when neither
// KeyFile nor Key is set and Endpoint is empty) is not covered: it can only resolve
// against a real GCP environment (metadata server, gcloud config, or
// GOOGLE_APPLICATION_CREDENTIALS), which an emulator cannot stand in for.
func (s *StorageSuite) TestGCP() {
	endpoint := s.startFakeGCS()

	s.Run("endpoint, no authentication", func() {
		s.assertBackupToStorage(&dto.Storage{GcpStorage: &dto.GcpStorage{
			BucketName: gcpBucket,
			Endpoint:   endpoint,
		}})
	})

	s.Run("key file, literal", func() {
		key := s.fakeServiceAccountKey(s.startFakeGCPTokenServer())
		keyFile := filepath.Join(s.T().TempDir(), "key.json")
		s.Require().NoError(os.WriteFile(keyFile, []byte(key), 0o600))

		s.assertBackupToStorage(&dto.Storage{GcpStorage: &dto.GcpStorage{
			BucketName: gcpBucket,
			Endpoint:   endpoint,
			KeyFile:    dto.Path(keyFile),
		}})
	})

	s.Run("key json, literal", func() {
		key := s.fakeServiceAccountKey(s.startFakeGCPTokenServer())

		s.assertBackupToStorage(&dto.Storage{GcpStorage: &dto.GcpStorage{
			BucketName: gcpBucket,
			Endpoint:   endpoint,
			Key:        redact.Secret(key),
		}})
	})

	s.Run("key json from secret agent", func() {
		agent := s.startSecretAgent(s.fakeServiceAccountKey(s.startFakeGCPTokenServer()))

		s.assertBackupToStorage(&dto.Storage{GcpStorage: &dto.GcpStorage{
			SecretAgentConfig: dto.SecretAgentConfig{SecretAgent: agent},
			BucketName:        gcpBucket,
			Endpoint:          endpoint,
			Key:               redact.Secret(secretRef()),
		}})
	})
}
