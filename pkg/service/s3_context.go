package service

import (
	"bytes"
	"context"
	"io"
	"log"
	"log/slog"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gopkg.in/yaml.v3"
)

type S3Context struct {
	ctx    context.Context
	client *s3.Client
	bucket string
	Path   string
}

func NewS3Context(storage *model.BackupStorage) *S3Context {
	// Load the SDK's configuration from environment and shared config, and
	// create the client with this.
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithSharedConfigProfile(*storage.S3Profile),
		config.WithRegion(*storage.S3Region))

	if err != nil {
		log.Fatalf("Failed to load S3 SDK configuration: %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if storage.S3EndpointOverride != nil && *storage.S3EndpointOverride != "" {
			o.BaseEndpoint = aws.String(*storage.S3EndpointOverride)
		}
		o.UsePathStyle = true
	})

	parsed, err := url.Parse(*storage.Path)
	if err != nil {
		log.Fatalf("Failed to parse S3 storage path: %v", err)
	}

	bucketName := parsed.Host

	// Check if the bucket exists
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		log.Fatalf("Error checking bucket %s existence %v", bucketName, err)
	}

	// Specify delimiter to list subfolders
	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucketName),
		Prefix:    aws.String("test-backup/backup"),
		Delimiter: aws.String("/"),
	}

	result, err := client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil
	}

	print(result)

	return &S3Context{
		ctx:    ctx,
		client: client,
		bucket: bucketName,
		Path:   parsed.Path,
	}
}

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
	bytes, err := io.ReadAll(result.Body)
	if err != nil {
		slog.Warn("Couldn't read object body of a file",
			"path", filePath, "err", err)
	}
	if err = yaml.Unmarshal(bytes, v); err != nil {
		slog.Warn("Failed unmarshal state file for backup",
			"path", filePath, "err", err)
	}
}

func (s *S3Context) writeFile(filePath string, v any) error {
	backupState, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(backupState)
	_, err = s.client.PutObject(s.ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(filePath),
		Body:   reader,
	})
	if err != nil {
		slog.Warn("Couldn't upload file", "path", filePath,
			"bucket", s.bucket, "err", err)
	}

	return err
}

func (s *S3Context) ListFiles(prefix string) ([]types.Object, error) {
	result, err := s.list(prefix, "")
	if err != nil {
		return nil, err
	}
	return result.Contents, nil
}

func (s *S3Context) ListFolders(prefix string) ([]types.CommonPrefix, error) {
	result, err := s.list(prefix, "/")
	if err != nil {
		return nil, err
	}
	return result.CommonPrefixes, nil
}

func (s *S3Context) list(prefix string, v string) (*s3.ListObjectsV2Output, error) {
	result, err := s.client.ListObjectsV2(s.ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(removeLeadingSlash(prefix)),
		Delimiter: aws.String(v),
	})

	if err != nil {
		slog.Warn("Couldn't list objects in folder", "prefix", prefix, "err", err)
		return nil, err
	}
	return result, nil
}

// minio works with slashes, but not aws.
func removeLeadingSlash(s string) string {
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}

func (s *S3Context) CleanDir(name string) {
	path := s.Path + "/" + name
	result, err := s.client.ListObjectsV2(s.ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(path),
		Delimiter: aws.String(""),
	})
	if err != nil {
		slog.Warn("Couldn't list files in directory", "path", path, "err", err)
	} else {
		for _, file := range result.Contents {
			_, err := s.client.DeleteObject(s.ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(s.bucket),
				Key:    file.Key,
			})
			if err != nil {
				slog.Debug("Couldn't delete file", "path", *file.Key, "err", err)
			}
		}
	}
}
