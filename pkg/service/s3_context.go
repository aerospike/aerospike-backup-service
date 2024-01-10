package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"gopkg.in/yaml.v3"
)

// S3Context is responsible for performing basic operations on S3.
type S3Context struct {
	ctx           context.Context
	client        *s3.Client
	bucket        string
	Path          string
	metadataCache *util.LoadingCache
}

// NewS3Context returns a new S3Context.
// Panics on any error during initialization.
func NewS3Context(storage *model.Storage) *S3Context {
	// Load the SDK's configuration from environment and shared config, and
	// create the client with this.
	ctx := context.TODO()
	cfg, err := createConfig(ctx, storage)
	if err != nil {
		panic(fmt.Sprintf("Failed to load S3 SDK configuration: %v", err))
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if storage.S3EndpointOverride != nil && *storage.S3EndpointOverride != "" {
			o.BaseEndpoint = aws.String(*storage.S3EndpointOverride)
		}
		o.UsePathStyle = true
	})

	parsed, err := url.Parse(*storage.Path)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse S3 storage path: %v", err))
	}

	bucketName := parsed.Host
	// Check if the bucket exists
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		panic(fmt.Sprintf("Error checking S3 bucket %s existence: %v", bucketName, err))
	}

	s := &S3Context{
		ctx:    ctx,
		client: client,
		bucket: bucketName,
		Path:   parsed.Path,
	}

	s.metadataCache = util.NewLoadingCache(ctx, func(path string) any {
		return s.readMetadata(path)
	})
	return s
}

func createConfig(ctx context.Context, storage *model.Storage) (aws.Config, error) {
	storage.SetDefaultProfile()
	return config.LoadDefaultConfig(
		ctx,
		config.WithSharedConfigProfile(*storage.S3Profile),
		config.WithRegion(*storage.S3Region),
	)
}

// readFile reads and decodes the YAML content from the given filePath into v.
func (s *S3Context) readFile(filePath string, v any) {
	result, err := s.client.GetObject(s.ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(removeLeadingSlash(filePath)),
	})
	if err != nil {
		slog.Warn("Failed to read file", "path", filePath)
		return
	}
	defer result.Body.Close()
	content, err := io.ReadAll(result.Body)
	if err != nil {
		slog.Warn("Couldn't read object body of a file",
			"path", filePath, "err", err)
	}
	if err = yaml.Unmarshal(content, v); err != nil {
		slog.Warn("Failed unmarshal state file for backup",
			"path", filePath, "err", err, "content", string(content))
	}
}

// writeFile writes v into filepath using the YAML format.
func (s *S3Context) writeFile(filePath string, v any) error {
	backupState, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(backupState)
	_, err = s.client.PutObject(s.ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(removeLeadingSlash(filePath)),
		Body:   reader,
	})
	if err != nil {
		slog.Warn("Couldn't upload file", "path", filePath,
			"bucket", s.bucket, "err", err)
		return err
	}
	slog.Debug("File written", "path", filePath, "bucket", s.bucket)
	return nil
}

// listFiles returns all files in the given s3 prefix path.
func (s *S3Context) listFiles(prefix string) ([]types.Object, error) {
	var nextContinuationToken *string
	result := make([]types.Object, 0)
	for {
		// By default, the action returns up to 1,000 key names.
		// It is necessary to repeat to collect all the items, if there are more.
		listOuptput, err := s.list(nextContinuationToken, prefix, "")
		if err != nil {
			return nil, err
		}
		result = append(result, listOuptput.Contents...)
		nextContinuationToken = listOuptput.NextContinuationToken
		if nextContinuationToken == nil {
			break
		}
	}
	return result, nil
}

// listFolders returns all subfolders in the given s3 prefix path.
func (s *S3Context) listFolders(prefix string) ([]types.CommonPrefix, error) {
	var nextContinuationToken *string
	result := make([]types.CommonPrefix, 0)
	for {
		// By default, the action returns up to 1,000 key names.
		// It is necessary to repeat to collect all the items, if there are more.
		listOuptput, err := s.list(nextContinuationToken, prefix, "/")
		if err != nil {
			return nil, err
		}
		result = append(result, listOuptput.CommonPrefixes...)
		nextContinuationToken = listOuptput.NextContinuationToken
		if nextContinuationToken == nil {
			break
		}
	}
	return result, nil
}

func (s *S3Context) list(continuationToken *string, prefix, v string) (*s3.ListObjectsV2Output, error) {
	result, err := s.client.ListObjectsV2(s.ctx, &s3.ListObjectsV2Input{
		Bucket:            aws.String(s.bucket),
		Prefix:            aws.String(removeLeadingSlash(prefix)),
		Delimiter:         aws.String(v),
		ContinuationToken: continuationToken,
	})

	if err != nil {
		slog.Warn("Couldn't list objects in folder", "prefix", prefix, "err", err)
		return nil, err
	}
	return result, nil
}

func removeLeadingSlash(s string) string {
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}

// CleanDir cleans the directory with the given name.
func (s *S3Context) CleanDir(name string) error {
	path := "s3://" + s.bucket + s.Path + "/" + name
	return s.DeleteFolder(path)
}

func (s *S3Context) GetMetadata(l types.CommonPrefix) *model.BackupMetadata {
	metadata, err := s.metadataCache.Get(*l.Prefix)
	if err == nil {
		return metadata.(*model.BackupMetadata)
	}
	return &model.BackupMetadata{}
}

func (s *S3Context) readMetadata(path string) *model.BackupMetadata {
	s3prefix := "s3://" + s.bucket
	metadataFilePath := strings.TrimPrefix(path, s3prefix) + metadataFile
	metadata := &model.BackupMetadata{}
	s.readFile(metadataFilePath, metadata)
	slog.Info("Read md", "path", path, "data", metadata)
	return metadata
}

func (s *S3Context) DeleteFolder(path string) error {
	parsed, err := url.Parse(path)
	if err != nil {
		return err
	}
	if parsed.Host != s.bucket {
		return fmt.Errorf("wrong bucket name for context: %s, expected: %s",
			parsed.Host, s.bucket)
	}

	result, err := s.client.ListObjectsV2(s.ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(removeLeadingSlash(parsed.Path)),
		Delimiter: aws.String(""),
	})
	if err != nil {
		slog.Warn("Couldn't list files in directory", "path", path, "err", err)
		return err
	}

	if len(result.Contents) == 0 {
		slog.Debug("No files to delete")
		return nil
	}

	for _, file := range result.Contents {
		_, err := s.client.DeleteObject(s.ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    file.Key,
		})
		if err != nil {
			slog.Debug("Couldn't delete file", "path", *file.Key, "err", err)
		}
	}
	return nil
}
