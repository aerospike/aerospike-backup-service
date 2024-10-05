package storage

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/aws/s3"
	"github.com/aws/aws-sdk-go-v2/config"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3StorageAccessor struct{}

func (a *S3StorageAccessor) supports(storage model.Storage) bool {
	_, ok := storage.(*model.S3Storage)
	return ok
}

func (a *S3StorageAccessor) createReader(
	ctx context.Context, storage model.Storage, path string, isFile bool, filter Validator,
) (backup.StreamingReader, error) {
	s3s := storage.(*model.S3Storage)
	client, err := getS3Client(ctx, s3s)
	if err != nil {
		return nil, err
	}
	opts := []s3.Opt{
		s3.WithValidator(filter),
		s3.WithNestedDir(),
	}
	fullPath := filepath.Join(s3s.Path, path)
	if isFile {
		opts = append(opts, s3.WithFile(fullPath))
	} else {
		opts = append(opts, s3.WithDir(fullPath))
	}
	return s3.NewReader(ctx, client, s3s.Bucket, opts...)
}

func (a *S3StorageAccessor) createWriter(
	ctx context.Context, storage model.Storage, path string, isFile, isRemoveFiles, withNested bool,
) (backup.Writer, error) {
	s3s := storage.(*model.S3Storage)
	client, err := getS3Client(ctx, s3s)
	if err != nil {
		return nil, err
	}
	fullPath := filepath.Join(s3s.Path, path)
	var opts []s3.Opt
	if isFile {
		opts = append(opts, s3.WithFile(fullPath))
	} else {
		opts = append(opts, s3.WithDir(fullPath))
	}
	if isRemoveFiles {
		opts = append(opts, s3.WithRemoveFiles())
	}
	if withNested {
		opts = append(opts, s3.WithNestedDir())
	}
	return s3.NewWriter(ctx, client, s3s.Bucket, opts...)
}

func init() {
	registerAccessor(&S3StorageAccessor{})
}

func getS3Client(ctx context.Context, s *model.S3Storage) (*awsS3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(s.S3Profile),
		config.WithRegion(s.S3Region),
	)
	if err != nil {
		return nil, err
	}

	client := awsS3.NewFromConfig(cfg, func(o *awsS3.Options) {
		if s.S3EndpointOverride != nil {
			o.BaseEndpoint = s.S3EndpointOverride
		}

		o.UsePathStyle = true

		if s.MaxConnsPerHost > 0 {
			o.HTTPClient = &http.Client{
				Transport: &http.Transport{
					MaxConnsPerHost: s.MaxConnsPerHost,
				},
			}
		}
	})

	return client, nil
}
