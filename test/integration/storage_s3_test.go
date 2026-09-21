//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// https://github.com/coollabsio/minio/pkgs/container/minio
	minioImage        = "ghcr.io/coollabsio/minio:RELEASE.2025-10-15T17-29-55Z"
	minioAPIPort      = "9000/tcp"
	minioRootUser     = "minioadmin"
	minioRootPassword = "minioadmin123" //nolint:gosec // test-only MinIO root credential, not a real secret

	s3Bucket = "abs-integration-test"
	s3Region = "us-east-1"

	s3AccessKeySecretName = "s3-access-key"
	s3SecretKeySecretName = "s3-secret-key"
)

// TestS3 exercises every way ABS can authenticate to an S3-compatible endpoint, against
// one shared MinIO container: static credentials (literal and Secret Agent), the AWS SDK
// default credential chain (env vars), and a named shared-config profile. IAM-role
// credentials are not covered: MinIO has no IMDS to hand them out, and faking one adds
// infrastructure without proving anything about ABS's own code.
func (s *StorageSuite) TestS3() {
	endpoint := s.startMinIO()

	s.Run("static credentials, literal values", func() {
		s.storageBackup(&dto.Storage{S3Storage: &dto.S3Storage{
			Bucket:             s3Bucket,
			S3Region:           s3Region,
			S3EndpointOverride: endpoint,
			AccessKeyID:        minioRootUser,
			SecretAccessKey:    minioRootPassword,
		}})
	})

	s.Run("static credentials from secret agent", func() {
		agent := s.startSecretAgentWithKeys(map[string]string{
			s3AccessKeySecretName: minioRootUser,
			s3SecretKeySecretName: minioRootPassword,
		})

		s.storageBackup(&dto.Storage{S3Storage: &dto.S3Storage{
			SecretAgentConfig:  dto.SecretAgentConfig{SecretAgent: agent},
			Bucket:             s3Bucket,
			S3Region:           s3Region,
			S3EndpointOverride: endpoint,
			AccessKeyID:        redact.Secret(secretRefKey(s3AccessKeySecretName)),
			SecretAccessKey:    redact.Secret(secretRefKey(s3SecretKeySecretName)),
		}})
	})

	s.Run("default credential chain via environment", func() {
		// No AccessKeyID/SecretAccessKey in the DTO: withCredentialsProvider leaves the
		// AWS SDK's default chain in place, which picks these up from the environment.
		s.T().Setenv("AWS_ACCESS_KEY_ID", minioRootUser)
		s.T().Setenv("AWS_SECRET_ACCESS_KEY", minioRootPassword)

		s.storageBackup(&dto.Storage{S3Storage: &dto.S3Storage{
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

		s.storageBackup(&dto.Storage{S3Storage: &dto.S3Storage{
			Bucket:             s3Bucket,
			S3Region:           s3Region,
			S3EndpointOverride: endpoint,
			S3Profile:          "test-profile",
		}})
	})
}

// startMinIO starts a MinIO container and creates the bucket the S3 tests write to.
// It returns the endpoint ABS should reach it at (dto.S3Storage.S3EndpointOverride).
func (s *StorageSuite) startMinIO() string {
	ctx := s.T().Context()

	container, err := testcontainers.Run(ctx, minioImage,
		testcontainers.WithExposedPorts(minioAPIPort),
		testcontainers.WithEnv(map[string]string{
			"MINIO_ROOT_USER":     minioRootUser,
			"MINIO_ROOT_PASSWORD": minioRootPassword,
		}),
		testcontainers.WithCmd("server", "/data"),
		testcontainers.WithWaitStrategy(wait.ForHTTP("/minio/health/live").WithPort(minioAPIPort)),
	)
	s.Require().NoError(err)
	s.cleanupContainer(container)

	host, err := container.Host(ctx)
	s.Require().NoError(err)

	mapped, err := container.MappedPort(ctx, minioAPIPort)
	s.Require().NoError(err)

	endpoint := fmt.Sprintf("http://%s:%d", host, mapped.Num())
	s.createS3Bucket(ctx, endpoint)

	return endpoint
}

// createS3Bucket creates the bucket the S3 tests share, authenticating as the MinIO
// root user directly (independent of whichever auth mode a given sub-test exercises).
func (s *StorageSuite) createS3Bucket(ctx context.Context, endpoint string) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(s3Region),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{AccessKeyID: minioRootUser, SecretAccessKey: minioRootPassword},
		}),
	)
	s.Require().NoError(err)

	client := awsS3.NewFromConfig(cfg, func(o *awsS3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	_, err = client.CreateBucket(ctx, &awsS3.CreateBucketInput{Bucket: aws.String(s3Bucket)})
	s.Require().NoError(err)
}
