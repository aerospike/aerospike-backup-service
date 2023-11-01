package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"time"

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
	ctx        context.Context
	client     *s3.Client
	bucket     string
	Path       string
	timestamps *util.LoadingCache
}

// NewS3Context returns a new S3Context.
// Panics on any error during initialization.
func NewS3Context(storage *model.BackupStorage) *S3Context {
	// Load the SDK's configuration from environment and shared config, and
	// create the client with this.
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithSharedConfigProfile(*storage.S3Profile),
		config.WithRegion(*storage.S3Region),
	)
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

	s.timestamps = util.NewLoadingCache(ctx, func(path string) (any, error) {
		return s.getCreationTime(path)
	})
	return s
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
			"path", filePath, "err", err)
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
		Key:    aws.String(filePath),
		Body:   reader,
	})
	if err != nil {
		slog.Warn("Couldn't upload file", "path", filePath,
			"bucket", s.bucket, "err", err)
	}

	return err
}

// listFiles returns all files in the given s3 prefix path.
func (s *S3Context) listFiles(prefix string) ([]types.Object, error) {
	result, err := s.list(prefix, "")
	if err != nil {
		return nil, err
	}
	return result.Contents, nil
}

// listFolders returns all subfolders in the given s3 prefix path.
func (s *S3Context) listFolders(prefix string) ([]types.CommonPrefix, error) {
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

func removeLeadingSlash(s string) string {
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}

// CleanDir cleans the directory with the given name.
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

func (s *S3Context) GetTime(l types.CommonPrefix) *time.Time {
	createTime, err := s.timestamps.Get(*l.Prefix)
	if err == nil {
		return createTime.(*time.Time)
	}
	return nil
}

func (s *S3Context) getCreationTime(path string) (*time.Time, error) {
	creationResult, err := s.client.ListObjects(s.ctx, &s3.ListObjectsInput{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(path),
	})

	if err != nil || len(creationResult.Contents) == 0 {
		return nil, fmt.Errorf("could not fetch timestamp %s", path)
	}

	// The creation date for the subfolder is same as of any file in it
	return creationResult.Contents[0].LastModified, nil
}
