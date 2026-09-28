//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
)

// TestS3 exercises every way ABS can authenticate to an S3-compatible endpoint, against
// one shared MinIO container: static credentials (literal and Secret Agent), the AWS SDK
// default credential chain (env vars), and a named shared-config profile. IAM-role
// credentials are not covered: MinIO has no IMDS to hand them out, and faking one adds
// infrastructure without proving anything about ABS's own code.
func (s *StorageSuite) TestS3() {
	endpoint := s.startMinIO()

	s.Run("static credentials, literal values", func() {
		s.assertBackupToStorage(&dto.Storage{S3Storage: &dto.S3Storage{
			Bucket:             s3Bucket,
			S3Region:           s3Region,
			S3EndpointOverride: endpoint,
			AccessKeyID:        minioRootUser,
			SecretAccessKey:    minioRootPassword,
		}})
	})

	s.Run("static credentials from secret agent", func() {
		agent := s.startSecretAgentWithKeys(map[string]string{
			"s3-access-key": minioRootUser,
			"s3-secret-key": minioRootPassword,
		})

		s.assertBackupToStorage(&dto.Storage{S3Storage: &dto.S3Storage{
			SecretAgentConfig:  dto.SecretAgentConfig{SecretAgent: agent},
			Bucket:             s3Bucket,
			S3Region:           s3Region,
			S3EndpointOverride: endpoint,
			AccessKeyID:        redact.Secret(secretRefKey("s3-access-key")),
			SecretAccessKey:    redact.Secret(secretRefKey("s3-secret-key")),
		}})
	})

	s.Run("default credential chain via environment", func() {
		// No AccessKeyID/SecretAccessKey in the DTO: withCredentialsProvider leaves the
		// AWS SDK's default chain in place, which picks these up from the environment.
		s.T().Setenv("AWS_ACCESS_KEY_ID", minioRootUser)
		s.T().Setenv("AWS_SECRET_ACCESS_KEY", minioRootPassword)

		s.assertBackupToStorage(&dto.Storage{S3Storage: &dto.S3Storage{
			Bucket:             s3Bucket,
			S3Region:           s3Region,
			S3EndpointOverride: endpoint,
		}})
	})

	s.Run("named profile via shared credentials file", func() {
		credentialsFile := filepath.Join(s.T().TempDir(), "credentials")
		contents := fmt.Sprintf("[test-profile]\naws_access_key_id = %s\naws_secret_access_key = %s\n",
			minioRootUser, minioRootPassword)
		s.Require().NoError(os.WriteFile(credentialsFile, []byte(contents), 0o600))
		s.T().Setenv("AWS_SHARED_CREDENTIALS_FILE", credentialsFile)

		s.assertBackupToStorage(&dto.Storage{S3Storage: &dto.S3Storage{
			Bucket:             s3Bucket,
			S3Region:           s3Region,
			S3EndpointOverride: endpoint,
			S3Profile:          "test-profile",
		}})
	})
}
