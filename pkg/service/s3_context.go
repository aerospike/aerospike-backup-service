package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"path/filepath"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"gopkg.in/yaml.v3"
)

const s3Protocol = "s3://"

// S3Context is responsible for performing basic operations on S3.
type S3Context struct {
	ctx           context.Context
	client        *s3.Client
	bucket        string
	Path          string
	metadataCache *util.LoadingCache[string, *model.BackupMetadata]
}

var _ StorageAccessor = (*S3Context)(nil)

// NewS3Context returns a new S3Context.
func NewS3Context(storage *model.S3Storage) *S3Context {
	// Load the SDK's configuration from environment and shared config, and
	// create the client with this.
	ctx := context.TODO()
	cfg := createConfig(ctx, storage)

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if storage.S3EndpointOverride != nil && *storage.S3EndpointOverride != "" {
			o.BaseEndpoint = aws.String(*storage.S3EndpointOverride)
		}
		o.UsePathStyle = true
	})

	// storage path is already validated.
	bucket, parsedPath, _ := util.ParseS3Path(*storage.Path)

	go checkBucket(ctx, client, bucket)

	s := &S3Context{
		ctx:    ctx,
		client: client,
		bucket: bucket,
		Path:   parsedPath,
	}

	s.metadataCache = util.NewLoadingCache(ctx, func(path string) (*model.BackupMetadata, error) {
		return s.readMetadata(path)
	})
	return s
}

// checkBucket verifies if the S3 bucket exists.
// As a side effect, it also ensures that the S3 service is available (network connectivity)
// and the provided credentials are valid.
// If the bucket doesn't exist at startup or AWS is unavailable, a warning is logged.
// However, it's not critical as the bucket only needs to be available during backup/restore operations.
// This function is executed in a goroutine to avoid blocking the initialization process.
func checkBucket(ctx context.Context, client *s3.Client, bucket string) {
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})

	if err != nil {
		slog.Warn("AWS S3 Bucket don't exist",
			slog.String("bucket", bucket),
			slog.Any("err", err))
	}
}

func createConfig(ctx context.Context, storage *model.S3Storage) aws.Config {
	storage.SetDefaultProfile()
	cfg, _ := config.LoadDefaultConfig(
		ctx,
		config.WithSharedConfigProfile(*storage.S3Profile),
		config.WithRegion(*storage.S3Region),
	)

	return cfg
}

func (s *S3Context) readBackupState(stateFilePath string, state *model.BackupState) error {
	return s.readFile(stateFilePath, state)
}

func (s *S3Context) readBackupDetails(path string, useCache bool) (model.BackupDetails, error) {
	var metadata *model.BackupMetadata
	var err error
	if useCache {
		metadata, err = s.getMetadataFromCache(path)
	} else {
		metadata, err = s.readMetadata(path)
	}
	if err != nil {
		return model.BackupDetails{}, err
	}
	return model.BackupDetails{
		BackupMetadata: *metadata,
		Key:            util.Ptr(s3Protocol + s.bucket + "/" + path),
	}, nil
}

func (s *S3Context) Read(filePath string) (io.ReadCloser, error) {
	result, err := s.client.GetObject(s.ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(filePath),
	})
	if err != nil {
		var opErr *smithy.OperationError
		if errors.As(err, &opErr) &&
			(strings.Contains(filePath, model.StateFileName) ||
				strings.Contains(filePath, metadataFile)) &&
			strings.Contains(opErr.Unwrap().Error(), "StatusCode: 404") {
			return nil, err
		}
		slog.Warn("Failed to read file", "path", filePath, "err", err)
		return nil, err
	}

	return result.Body, nil
}

// readFile reads and decodes the YAML content from the given filePath into v.
func (s *S3Context) readFile(filePath string, v any) error {
	content, err := s.Read(filePath)
	if err != nil {
		return err
	}
	all, _ := io.ReadAll(content)
	if err = yaml.Unmarshal(all, v); err != nil {
		slog.Warn("Failed unmarshal state file for backup",
			"path", filePath, "err", err, "content", string(all))
		return err
	}
	return nil
}

func (s *S3Context) Write(filePath string, data []byte) error {
	logger := slog.Default().With(slog.String("path", filePath),
		slog.String("bucket", s.bucket))
	_, err := s.client.PutObject(s.ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(filePath),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		logger.Warn("Couldn't upload file", slog.Any("err", err))
		return err
	}
	logger.Debug("File written")
	return nil
}

// lsFiles returns all files in the given s3 prefix Path.
func (s *S3Context) lsFiles(prefix string) ([]string, error) {
	var result []string

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(strings.TrimSuffix(prefix, "/") + "/"),
	}

	paginator := s3.NewListObjectsV2Paginator(s.client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(s.ctx)
		if err != nil {
			slog.Warn("Couldn't list objects in folder",
				slog.String("prefix", prefix),
				slog.Any("err", err))
			return nil, fmt.Errorf("error listing objects: %w", err)
		}

		for _, item := range page.Contents {
			if item.Key != nil {
				result = append(result, *item.Key)
			}
		}
	}

	return result, nil
}

// lsDir returns all subfolders in the given s3 prefix Path.
func (s *S3Context) lsDir(prefix string, after *string) ([]string, error) {
	var result []string

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(strings.TrimSuffix(prefix, "/") + "/"),
		Delimiter: aws.String("/"),
	}

	if after != nil {
		startAfter := *after

		if !strings.HasPrefix(startAfter, prefix) {
			startAfter = path.Join(prefix, startAfter)
		}

		input.StartAfter = &startAfter
	}

	paginator := s3.NewListObjectsV2Paginator(s.client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(s.ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing objects: %w", err)
		}

		for _, p := range page.CommonPrefixes {
			if p.Prefix == nil {
				continue
			}
			subfolder := strings.TrimSuffix(*p.Prefix, "/")
			if subfolder != "" {
				result = append(result, subfolder)
			}
		}
	}

	return result, nil
}

func (s *S3Context) getMetadataFromCache(prefix string) (*model.BackupMetadata, error) {
	metadata, err := s.metadataCache.Get(prefix)
	if err != nil {
		return nil, err
	}
	return metadata, nil
}

func (s *S3Context) readMetadata(path string) (*model.BackupMetadata, error) {
	metadata := &model.BackupMetadata{}
	metadataFilePath := filepath.Join(path, metadataFile)
	err := s.readFile(metadataFilePath, metadata)
	if err != nil {
		return nil, err
	}
	slog.Debug("Read metadata file", "path", path, "data", metadata)
	return metadata, nil
}

func (s *S3Context) DeleteFolder(folder string) error {
	var continuationToken *string
	prefix := strings.TrimSuffix(folder, "/") + "/"

	for {
		listResponse, err := s.client.ListObjectsV2(s.ctx, &s3.ListObjectsV2Input{
			Bucket:            &s.bucket,
			Prefix:            &prefix,
			ContinuationToken: continuationToken,
		})

		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		for _, p := range listResponse.Contents {
			if p.Key == nil {
				continue
			}

			_, err = s.client.DeleteObject(s.ctx, &s3.DeleteObjectInput{
				Bucket: &s.bucket,
				Key:    p.Key,
			})
			if err != nil {
				return fmt.Errorf("failed to delete object %s: %w", *p.Key, err)
			}
		}

		continuationToken = listResponse.NextContinuationToken
		if continuationToken == nil {
			break
		}
	}

	return nil
}

func (s *S3Context) wrapWithPrefix(path string) *string {
	result := s3Protocol + s.bucket + "/" + path + "/"
	return &result
}

func (s *S3Context) ValidateStorageContainsBackup() (uint64, error) {
	files, err := s.lsFiles(s.Path)
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		return 0, fmt.Errorf("given path %s does not exist", s.Path)
	}

	metadata, err := s.readMetadata(s.Path)
	if err != nil {
		return 0, err
	}
	for _, file := range files {
		if strings.HasSuffix(file, ".asb") {
			return metadata.RecordCount, nil
		}
	}
	return 0, fmt.Errorf("no backup files found in %s", s.Path)
}
