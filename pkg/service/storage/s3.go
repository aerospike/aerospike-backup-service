package storage

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/storage/aws/s3"
	"github.com/aerospike/backup-go/io/storage/options"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3StorageAccessor struct {
	clientMap collections.Cache[*model.S3Storage, *awsS3.Client]
	resolver  secrets.Resolver
}

func NewS3StorageAccessor(resolver secrets.Resolver) *S3StorageAccessor {
	accessor := &S3StorageAccessor{
		resolver: resolver,
	}
	accessor.clientMap = collections.NewLoadingCache[*model.S3Storage, *awsS3.Client](
		accessor.getS3Client,
		clientCacheTTL,
	)
	return accessor
}

func (a *S3StorageAccessor) supports(storage model.Storage) bool {
	_, ok := storage.(*model.S3Storage)
	return ok
}

func (a *S3StorageAccessor) createReader(
	ctx context.Context,
	storage model.Storage,
	opts ...options.Opt,
) (backup.StreamingReader, error) {
	s3s := storage.(*model.S3Storage)
	client, err := a.clientMap.Get(ctx, s3s)
	if err != nil {
		return nil, err
	}
	opts = append(opts, options.WithRetryPolicy(&model.StorageRetryPolicy.RetryPolicy))

	return s3.NewReader(ctx, client, s3s.Bucket, opts...)
}

func (a *S3StorageAccessor) createWriter(
	ctx context.Context, storage model.Storage, opts ...options.Opt,
) (backup.Writer, error) {
	s3s := storage.(*model.S3Storage)
	client, err := a.clientMap.Get(ctx, s3s)
	if err != nil {
		return nil, err
	}

	if s3s.MinPartSize != nil {
		opts = append(opts, options.WithChunkSize(*s3s.MinPartSize))
	}

	return s3.NewWriter(ctx, client, s3s.Bucket, opts...)
}

func (a *S3StorageAccessor) getS3Client(ctx context.Context, s *model.S3Storage) (*awsS3.Client, error) {
	credentialsProvider, err := a.withCredentialsProvider(ctx, s.Auth)
	if err != nil {
		return nil, err
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		credentialsProvider,
		config.WithSharedConfigProfile(s.S3Profile),
		config.WithRegion(s.S3Region),

		// use an adaptive mode for more aggressive retries
		config.WithRetryer(func() aws.Retryer {
			return retry.NewAdaptiveMode(func(o *retry.AdaptiveModeOptions) {
				o.StandardOptions = append(o.StandardOptions,
					func(so *retry.StandardOptions) {
						so.MaxAttempts = int(model.StorageRetryPolicy.MaxRetries)
						so.MaxBackoff = model.StorageRetryPolicy.MaxBackoffDuration
						so.Backoff = retry.NewExponentialJitterBackoff(model.StorageRetryPolicy.MaxBackoffDuration)
					})
			})
		}),
	)

	if err != nil {
		return nil, err
	}

	client := awsS3.NewFromConfig(cfg, func(o *awsS3.Options) {
		if s.S3EndpointOverride != "" {
			o.BaseEndpoint = aws.String(s.S3EndpointOverride)
		}

		o.UsePathStyle = true

		transport := &http.Transport{
			// The DialContext function is responsible for creating the underlying TCP connection.
			DialContext: (&net.Dialer{
				// Timeout for establishing a new TCP connection. If a connection isn't
				// established within this duration, the request will fail.
				Timeout: 30 * time.Second,

				//  KeepAlive specifies the interval between keep-alive probes for an active network connection.
				// Setting this helps prevent network intermediaries (like NATs, firewalls) from dropping
				// the connection during long transfers due to inactivity.
				KeepAlive: 30 * time.Second,
			}).DialContext,

			MaxConnsPerHost:     ptr.ValueOrZero(s.MaxConnsPerHost),
			IdleConnTimeout:     120 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			ReadBufferSize:      64 * 1024,
		}

		o.HTTPClient = &http.Client{
			Transport: transport,
			Timeout:   model.StorageRetryPolicy.MaxRequestTimeout,
		}
	})

	if err := checkS3Connectivity(ctx, client, s.Bucket); err != nil {
		return nil, err
	}

	return client, nil
}

func (a *S3StorageAccessor) withCredentialsProvider(
	ctx context.Context,
	auth *model.S3Authentication,
) (config.LoadOptionsFunc, error) {
	if auth == nil {
		return func(*config.LoadOptions) error {
			return nil // No-op implementation
		}, nil
	}

	keyID, err := a.resolver.Resolve(ctx, auth.SecretAgent, auth.KeyIDSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve key ID: %w", err)
	}

	accessKey, err := a.resolver.Resolve(ctx, auth.SecretAgent, auth.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve access key: %w", err)
	}

	return config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID: keyID, SecretAccessKey: accessKey,
		},
	}), nil
}

func checkS3Connectivity(ctx context.Context, client *awsS3.Client, bucket string) error {
	ctx, cancel := context.WithTimeout(ctx, connectivityTimeout)
	defer cancel()

	_, err := client.HeadBucket(ctx, &awsS3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("s3 storage connectivity check failed: %w", err)
	}

	_, err = client.ListObjectsV2(ctx, &awsS3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return fmt.Errorf("s3 storage read permission check failed: %w", err)
	}

	checkS3MultipartUpload(ctx, client, bucket)

	return nil
}

const s3uploadPermissionWarnMsg = "s3 storage upload permission check failed; backup writes may fail at runtime"

func checkS3MultipartUpload(ctx context.Context, client *awsS3.Client, bucket string) {
	createOutput, err := client.CreateMultipartUpload(ctx, &awsS3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(connectivityProbeKey),
	})
	if err != nil {
		slog.Warn(s3uploadPermissionWarnMsg, slog.String("bucket", bucket), attr.Error(err))
		return
	}

	uploadID := createOutput.UploadId
	etag, err := uploadS3ProbePart(ctx, client, bucket, uploadID)
	if err != nil {
		abortS3MultipartUpload(ctx, client, bucket, uploadID)
		return
	}

	if err = completeS3MultipartUpload(ctx, client, bucket, uploadID, etag); err != nil {
		abortS3MultipartUpload(ctx, client, bucket, uploadID)
		return
	}

	deleteS3ProbeObject(ctx, client, bucket)
}

func uploadS3ProbePart(ctx context.Context, client *awsS3.Client, bucket string, uploadID *string) (string, error) {
	uploadOutput, err := client.UploadPart(ctx, &awsS3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(connectivityProbeKey),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader([]byte{}),
	})
	if err != nil {
		slog.Warn(s3uploadPermissionWarnMsg, slog.String("bucket", bucket), attr.Error(err))
		return "", err
	}

	return *uploadOutput.ETag, nil
}

func completeS3MultipartUpload(
	ctx context.Context,
	client *awsS3.Client,
	bucket string,
	uploadID *string,
	etag string,
) error {
	_, err := client.CompleteMultipartUpload(ctx, &awsS3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(connectivityProbeKey),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{
					ETag:       aws.String(etag),
					PartNumber: aws.Int32(1),
				},
			},
		},
	})
	if err != nil {
		slog.Warn(s3uploadPermissionWarnMsg, slog.String("bucket", bucket), attr.Error(err))
	}

	return err
}

func abortS3MultipartUpload(ctx context.Context, client *awsS3.Client, bucket string, uploadID *string) {
	_, _ = client.AbortMultipartUpload(ctx, &awsS3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(connectivityProbeKey),
		UploadId: uploadID,
	})
}

func deleteS3ProbeObject(ctx context.Context, client *awsS3.Client, bucket string) {
	_, err := client.DeleteObject(ctx, &awsS3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(connectivityProbeKey),
	})
	if err != nil {
		slog.Warn("s3 storage delete permission check failed; backup writes or cleanup may fail at runtime",
			slog.String("bucket", bucket),
			attr.Error(err),
		)
	}
}
