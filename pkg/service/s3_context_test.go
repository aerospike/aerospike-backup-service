//go:build !ci

package service

import (
	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/smithy-go/ptr"
	"testing"
)

func TestS3Context_DeleteFile(t *testing.T) {
	context := NewS3Context(&model.Storage{
		Type:               model.S3,
		Path:               ptr.String("s3://as-backup-bucket/storage1"),
		S3Profile:          ptr.String("minio"),
		S3Region:           ptr.String("eu-central-1"),
		S3EndpointOverride: ptr.String("http://localhost:9000"),
	})

	context.writeFile("incremental/file.txt", "data")
	context.writeFile("incremental/file2.txt", "data")

	if files, _ := context.listFiles("incremental"); len(files) != 2 {
		t.Error("files not created")
	}

	context.CleanDir("incremental")

	if files, _ := context.listFiles("incremental"); len(files) > 0 {
		t.Error("files not deleted")
	}
}
