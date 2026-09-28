//go:build integration

package integration

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The MinIO account and the bucket startMinIO creates.
const (
	minioRootUser     = "minioadmin"
	minioRootPassword = "minioadmin123"

	s3Bucket = "abs-integration-test"
	s3Region = "us-east-1"
)

const (
	// https://github.com/coollabsio/minio/pkgs/container/minio
	minioImage   = "ghcr.io/coollabsio/minio:RELEASE.2025-10-15T17-29-55Z"
	minioAPIPort = "9000/tcp"
)

// startMinIO starts a MinIO container and creates s3Bucket in it. It returns the
// endpoint ABS reaches it at (dto.S3Storage.S3EndpointOverride).
func (s *Suite) startMinIO() string {
	ctx := s.T().Context()

	minio, err := testcontainers.Run(ctx, minioImage,
		testcontainers.WithExposedPorts(minioAPIPort),
		testcontainers.WithEnv(map[string]string{
			"MINIO_ROOT_USER":     minioRootUser,
			"MINIO_ROOT_PASSWORD": minioRootPassword,
		}),
		testcontainers.WithCmd("server", "/data"),
		testcontainers.WithWaitStrategy(wait.ForHTTP("/minio/health/live").WithPort(minioAPIPort)),
	)
	s.Require().NoError(err)
	s.terminateOnCleanup(minio, "minio")

	host, err := minio.Host(ctx)
	s.Require().NoError(err)

	mapped, err := minio.MappedPort(ctx, minioAPIPort)
	s.Require().NoError(err)

	endpoint := fmt.Sprintf("http://%s:%d", host, mapped.Num())
	s.createS3Bucket(ctx, endpoint)

	return endpoint
}

// createS3Bucket authenticates as the MinIO root user directly, independent of
// whichever auth mode a test gives ABS.
func (s *Suite) createS3Bucket(ctx context.Context, endpoint string) {
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
