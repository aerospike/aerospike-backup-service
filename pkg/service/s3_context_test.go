//go:build !ci

package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aws/smithy-go/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var contexts []S3Context
var minioContext *S3Context

func init() {
	minioContext = NewS3Context(&model.Storage{
		Type:               model.S3,
		Path:               ptr.String("s3://as-backup-bucket/storageMinio"),
		S3Profile:          ptr.String("minio"),
		S3Region:           ptr.String("eu-central-1"),
		S3EndpointOverride: ptr.String("http://localhost:9000"),
	})
	s3Context := NewS3Context(&model.Storage{
		Type:     model.S3,
		Path:     ptr.String("s3://as-backup-integration-test/storageAws"),
		S3Region: ptr.String("eu-central-1"),
	})
	if minioContext != nil && s3Context != nil {
		contexts = []S3Context{
			*minioContext,
			*s3Context,
		}
	}
}

func TestReadWriteState(t *testing.T) {
	for _, context := range contexts {
		t.Run(context.path, func(t *testing.T) {
			runReadWriteState(t, context)
		})
	}
}

func runReadWriteState(t *testing.T, context S3Context) {
	t.Helper()
	metadataWrite := model.BackupMetadata{
		Namespace: "testNS",
		Created:   time.Now(),
	}
	_ = context.writeYaml("backup_path/"+metadataFile, metadataWrite)
	metadataRead := model.BackupMetadata{}

	_ = context.readFile("backup_path/"+metadataFile, &metadataRead)
	if metadataWrite.Namespace != metadataRead.Namespace {
		t.Errorf("namespace different, expected %s, got %s",
			metadataWrite.Namespace, metadataRead.Namespace)
	}
	if !metadataWrite.Created.Equal(metadataRead.Created) {
		t.Errorf("created different, expected %v, got %v",
			metadataWrite.Created, metadataRead.Created)
	}
}

func TestS3Context_DeleteFile(t *testing.T) {
	if contexts == nil {
		t.Skip("contexts is nil")
	}
	for _, context := range contexts {
		t.Run(context.path, func(t *testing.T) {
			runDeleteFileTest(t, context)
		})
	}
}

func runDeleteFileTest(t *testing.T, context S3Context) {
	t.Helper()
	_ = context.writeYaml("incremental/file.txt", "data")
	_ = context.writeYaml("incremental/file2.txt", "data")

	if files, _ := context.lsFiles("incremental"); len(files) != 2 {
		t.Error("files not created")
	}

	// DeleteFolder requires full path
	_ = context.DeleteFolder("incremental")

	if files, _ := context.lsFiles("incremental"); len(files) > 0 {
		t.Error("files not deleted")
	}
}

func TestS3Context_DeleteFolder(t *testing.T) {
	if contexts == nil {
		t.Skip("contexts is nil")
	}
	for _, context := range contexts {
		t.Run(context.path, func(t *testing.T) {
			runDeleteFolderTest(t, context)
		})
	}
}

func runDeleteFolderTest(t *testing.T, context S3Context) {
	t.Helper()
	parent := "storage1/minioIncremental"
	folder1 := parent + "/source-ns1"
	folder2 := parent + "/source-ns16"
	_ = context.writeYaml(folder1+"/file1.txt", "data")
	_ = context.writeYaml(folder2+"/file2.txt", "data")

	err := context.DeleteFolder(folder1)
	if err != nil {
		t.Error("Error deleting", err)
	}

	listFiles1, _ := context.lsFiles(folder1)
	if len(listFiles1) != 0 {
		t.Error("file 1 not deleted")
	}
	listFiles2, _ := context.lsFiles(folder2)
	if len(listFiles2) != 1 {
		t.Error("file 2 was deleted")
	}

	err = context.DeleteFolder(parent)
	if err != nil {
		t.Error("Error deleting", err)
	}

	listFiles3, _ := context.lsFiles(folder2)
	if len(listFiles3) != 0 {
		t.Error("file 2 not deleted")
	}
}

func TestS3Context_LsDir(t *testing.T) {
	parent := "backups/incremental"
	_ = minioContext.DeleteFolder(parent)
	folder1 := parent + "/1000"
	folder2 := parent + "/2000"
	folder3 := parent + "/3000"
	_ = minioContext.writeYaml(folder1+"/file1.txt", "data")
	_ = minioContext.writeYaml(folder2+"/file2.txt", "data")
	_ = minioContext.writeYaml(folder3+"/file2.txt", "data")
	after := "2000"
	dir, err := minioContext.lsDir(parent, &after)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(dir))
}

func TestS3Context_lsFiles(t *testing.T) {
	testCases := []struct {
		name          string
		setup         func() error
		prefix        string
		expectedFiles []string
	}{
		{
			name: "Single file",
			setup: func() error {
				return minioContext.writeYaml("test-prefix/file1.txt", "content")
			},
			prefix:        "test-prefix",
			expectedFiles: []string{"test-prefix/file1.txt"},
		},
		{
			name: "Multiple files",
			setup: func() error {
				if err := minioContext.writeYaml("test-prefix/file1.txt", "content"); err != nil {
					return err
				}
				if err := minioContext.writeYaml("test-prefix/file2.txt", "content"); err != nil {
					return err
				}
				return minioContext.writeYaml("test-prefix/subdir/file3.txt", "content")
			},
			prefix: "test-prefix",
			expectedFiles: []string{"test-prefix/file1.txt",
				"test-prefix/file2.txt",
				"test-prefix/subdir/file3.txt"},
		},
		{
			name: "Many files",
			setup: func() error {
				for i := 0; i < 3000; i++ {
					filename := fmt.Sprintf("test-prefix/file%04d.txt", i)
					if err := minioContext.writeYaml(filename, "content"); err != nil {
						return err
					}
				}
				return nil
			},
			prefix: "test-prefix",
			expectedFiles: func() []string {
				files := make([]string, 3000)
				for i := 0; i < len(files); i++ {
					files[i] = fmt.Sprintf("test-prefix/file%04d.txt", i)
				}
				return files
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.setup()
			require.NoError(t, err)

			files, err := minioContext.lsFiles(tc.prefix)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tc.expectedFiles, files)

			err = minioContext.DeleteFolder(tc.prefix)
			require.NoError(t, err)
		})
	}
}
