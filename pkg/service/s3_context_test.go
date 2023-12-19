//go:build !ci

package service

import (
	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/smithy-go/ptr"

	"testing"
)

var contexts = []S3Context{
	*NewS3Context(&model.Storage{
		Type:               model.S3,
		Path:               ptr.String("s3://as-backup-bucket/storageMinio"),
		S3Profile:          ptr.String("minio"),
		S3Region:           ptr.String("eu-central-1"),
		S3EndpointOverride: ptr.String("http://localhost:9000"),
	}),
	*NewS3Context(&model.Storage{
		Type:     model.S3,
		Path:     ptr.String("s3://as-backup-integration-test/storageAws"),
		S3Region: ptr.String("eu-central-1"),
	}),
}

func TestS3Context_CleanDir(t *testing.T) {
	for _, context := range contexts {
		t.Run(context.Path, func(t *testing.T) {
			runCleanDirTest(t, context)
		})
	}
}

func runCleanDirTest(t *testing.T, context S3Context) {
	context.writeFile(context.Path+"/incremental/file.txt", "data")
	context.writeFile(context.Path+"/incremental/file2.txt", "data")

	if files, _ := context.listFiles(context.Path + "/incremental"); len(files) != 2 {
		t.Error("files not created")
	}

	context.CleanDir("incremental") // clean is public function, so "storage1" is appended inside

	if files, _ := context.listFiles(context.Path + "/incremental"); len(files) > 0 {
		t.Error("files not deleted")
	}
}

func TestS3Context_DeleteFile(t *testing.T) {
	for _, context := range contexts {
		t.Run(context.Path, func(t *testing.T) {
			runDeleteFileTest(t, context)
		})
	}
}

func runDeleteFileTest(t *testing.T, context S3Context) {
	context.writeFile(context.Path+"/incremental/file.txt", "data")
	context.writeFile(context.Path+"/incremental/file2.txt", "data")

	if files, _ := context.listFiles(context.Path + "/incremental"); len(files) != 2 {
		t.Error("files not created")
	}

	// DeleteFile require full path
	context.DeleteFile("s3://" + context.bucket + context.Path + "/incremental/file.txt")
	context.DeleteFile("s3://" + context.bucket + context.Path + "/incremental/file2.txt")

	if files, _ := context.listFiles(context.Path + "/incremental"); len(files) > 0 {
		t.Error("files not deleted")
	}
}
