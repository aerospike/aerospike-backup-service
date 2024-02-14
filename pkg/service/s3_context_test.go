package service

import (
    "testing"
    "time"

    "github.com/aerospike/backup/pkg/model"
    "github.com/aws/smithy-go/ptr"
)

var contexts []S3Context

func init() {
    minioContext, _ := NewS3Context(&model.Storage{
        Type:               model.S3,
        Path:               ptr.String("s3://as-backup-bucket/storageMinio"),
        S3Profile:          ptr.String("minio"),
        S3Region:           ptr.String("eu-central-1"),
        S3EndpointOverride: ptr.String("http://localhost:9000"),
    })
    s3Context, _ := NewS3Context(&model.Storage{
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
    metadataWrite := model.BackupMetadata{
        Namespace: "testNS",
        Created:   time.Now(),
    }
    context.writeYaml("backup_path/"+metadataFile, metadataWrite)
    metadataRead := model.BackupMetadata{}

    context.readFile("backup_path/"+metadataFile, &metadataRead)
    if metadataWrite.Namespace != metadataRead.Namespace {
        t.Errorf("namespace different, expected %s, got %s", metadataWrite.Namespace, metadataRead.Namespace)
    }
    if !metadataWrite.Created.Equal(metadataRead.Created) {
        t.Errorf("created different, expected %v, got %v", metadataWrite.Created, metadataRead.Created)
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
    context.writeYaml("incremental/file.txt", "data")
    context.writeYaml("incremental/file2.txt", "data")

    if files, _ := context.listFiles("incremental"); len(files) != 2 {
        t.Error("files not created")
    }

    // DeleteFolder requires full path
    context.DeleteFolder("incremental")

    if files, _ := context.listFiles("incremental"); len(files) > 0 {
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
    parent := "storage1/minioIncremental"
    folder1 := parent + "/source-ns1"
    folder2 := parent + "/source-ns16"
    context.writeYaml(folder1+"/file1.txt", "data")
    context.writeYaml(folder2+"/file2.txt", "data")

    err := context.DeleteFolder(folder1)
    if err != nil {
        t.Error("Error deleting", err)
    }

    listFiles1, _ := context.listFiles(folder1)
    if len(listFiles1) != 0 {
        t.Error("file 1 not deleted")
    }
    listFiles2, _ := context.listFiles(folder2)
    if len(listFiles2) != 1 {
        t.Error("file 2 was deleted")
    }

    err = context.DeleteFolder(parent)
    if err != nil {
        t.Error("Error deleting", err)
    }

    listFiles3, _ := context.listFiles(folder2)
    if len(listFiles3) != 0 {
        t.Error("file 2 not deleted")
    }
}
