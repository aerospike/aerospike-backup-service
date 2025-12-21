package storage

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
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
)

type S3StorageAccessor struct {
	clientMap *collections.LoadingCache[*model.S3Storage, *awsS3.Client]
}

func NewS3StorageAccessor() *S3StorageAccessor {
	return &S3StorageAccessor{
		clientMap: collections.NewLoadingCache[*model.S3Storage, *awsS3.Client](context.Background(), getS3Client),
	}
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
	client, err := a.clientMap.GetWithContext(ctx, s3s)
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
	client, err := a.clientMap.GetWithContext(ctx, s3s)
	if err != nil {
		return nil, err
	}

	if s3s.MinPartSize != nil {
		opts = append(opts, options.WithChunkSize(*s3s.MinPartSize))
	}

	return s3.NewWriter(ctx, client, s3s.Bucket, opts...)
}

func init() {
	registerAccessor(NewS3StorageAccessor())
}

func getS3Client(ctx context.Context, s *model.S3Storage) (*awsS3.Client, error) {
	credentialsProvider, err := withCredentialsProvider(s.Auth)
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
		if s.S3EndpointOverride != nil && *s.S3EndpointOverride != "" {
			o.BaseEndpoint = s.S3EndpointOverride
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

func withCredentialsProvider(a *model.S3Authentication) (config.LoadOptionsFunc, error) {
	if a == nil {
		return func(*config.LoadOptions) error {
			return nil // No-op implementation
		}, nil
	}

	keyID, accessKey, err := a.ReadSecrets()
	if err != nil {
		return nil, err
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

	return nil
}
