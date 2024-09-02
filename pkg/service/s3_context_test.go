//go:build !ci

package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/internal/server/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aws/smithy-go/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var contexts []S3Context
var minioContext *S3Context
var content = make([]byte, 4)

func init() {
	minioContext = NewS3Context(&model.Storage{
		Type:               model.S3,
		Path:               ptr.String("s3://as-backup-bucket/storageMinio"),
		S3Profile:          ptr.String("minio"),
		S3Region:           ptr.String("eu-central-1"),
		S3EndpointOverride: ptr.String("http://localhost:9000"),
	})
	if minioContext != nil {
		contexts = []S3Context{
			*minioContext,
		}
	}
}

func TestReadWriteState(t *testing.T) {
	for _, context := range contexts {
		t.Run(context.Path, func(t *testing.T) {
			runReadWriteState(t, context)
		})
	}
}

func runReadWriteState(t *testing.T, context S3Context) {
	t.Helper()
	metadataWrite := dto.BackupMetadata{
		Namespace: "testNS",
		Created:   time.Now(),
	}
	data, _ := yaml.Marshal(metadataWrite)
	_ = context.Write("backup_path/"+metadataFile, data)
	metadataRead := dto.BackupMetadata{}

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
		t.Run(context.Path, func(t *testing.T) {
			runDeleteFileTest(t, context)
		})
	}
}

func runDeleteFileTest(t *testing.T, context S3Context) {
	t.Helper()
	_ = context.Write("incremental/file.txt", content)
	_ = context.Write("incremental/file2.txt", content)

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
		t.Run(context.Path, func(t *testing.T) {
			runDeleteFolderTest(t, context)
		})
	}
}

func runDeleteFolderTest(t *testing.T, context S3Context) {
	t.Helper()
	parent := "storage1/minioIncremental"
	folder1 := parent + "/source-ns1"
	folder2 := parent + "/source-ns16"
	_ = context.Write(folder1+"/file1.txt", content)
	_ = context.Write(folder2+"/file2.txt", content)

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
	_ = minioContext.Write(folder1+"/file1.txt", make([]byte, 4))
	_ = minioContext.Write(folder2+"/file2.txt", make([]byte, 4))
	_ = minioContext.Write(folder3+"/file2.txt", make([]byte, 4))
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
				return minioContext.Write("test-prefix/file1.txt", content)
			},
			prefix:        "test-prefix",
			expectedFiles: []string{"test-prefix/file1.txt"},
		},
		{
			name: "Multiple files",
			setup: func() error {
				if err := minioContext.Write("test-prefix/file1.txt", content); err != nil {
					return err
				}
				if err := minioContext.Write("test-prefix/file2.txt", content); err != nil {
					return err
				}
				return minioContext.Write("test-prefix/subdir/file3.txt", content)
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
					if err := minioContext.Write(filename, content); err != nil {
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
			err := minioContext.DeleteFolder(tc.prefix)
			require.NoError(t, err)

			err = tc.setup()
			require.NoError(t, err)

			files, err := minioContext.lsFiles(tc.prefix)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tc.expectedFiles, files)

			err = minioContext.DeleteFolder(tc.prefix)
			require.NoError(t, err)
		})
	}
}
