package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
)

var jsonExamples = map[string]any{
	"ClustersResponse": []dto.AerospikeCluster{
		{
			SeedNodes: []dto.SeedNode{{
				HostName: "host.docker.internal", Port: 3000},
			},
			Credentials: &dto.Credentials{
				User:     util.Ptr("user"),
				Password: util.Ptr("password"),
			},
		},
	},
	"RoutinesResponse": map[string]dto.BackupRoutine{
		"routine1": {
			BackupPolicy:  "keepFilesPolicy",
			SourceCluster: "absDefaultCluster",
			Storage:       "local",
			IntervalCron:  "@yearly",
			Namespaces:    []string{"test-namespace"},
		},
		"routine2": {
			BackupPolicy:     "removeFilesPolicy",
			SourceCluster:    "absDefaultCluster",
			Storage:          "local",
			IntervalCron:     "@monthly",
			IncrIntervalCron: "@daily",
			Namespaces:       []string{"test-namespace"},
			SetList:          []string{"backupSet"},
			BinList:          []string{"backupBin"},
		},
	},
	"StorageResponse": map[string]dto.Storage{
		"local": {
			LocalStorage: &dto.LocalStorage{
				Path: "backups",
			},
		},
		"aws-s3": {
			S3Storage: &dto.S3Storage{
				Bucket:   "as-backup-bucket",
				Path:     "backups",
				S3Region: "eu-central-1",
			},
		},
		"gcp-gcs": {
			GcpStorage: &dto.GcpStorage{
				Path:       "backups",
				KeyFile:    "key-file.json",
				BucketName: "gcp-backup-bucket",
				Endpoint:   "http://127.0.0.1:9020",
			},
		},
		"azure-blob-storage": {
			AzureStorage: &dto.AzureStorage{
				Path:          "backups",
				Endpoint:      "http://127.0.0.1:6000/devstoreaccount1",
				AccountName:   "devstoreaccount1",
				ContainerName: "testcontainer",
				AccountKey:    "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
			},
		},
	},
	"FullBackupsResponse": map[string][]dto.BackupDetails{
		"routine1": {{
			BackupMetadata: dto.BackupMetadata{
				Created:             time.Date(2024, 01, 01, 12, 0, 0, 0, time.UTC),
				Namespace:           "source-ns1",
				ByteCount:           480_000,
				FileCount:           1,
				UDFCount:            1,
				RecordCount:         42,
				SecondaryIndexCount: 5,
			},
			Key: "routine1/backup/1704110400000/source-ns1",
			Storage: &dto.Storage{
				S3Storage: &dto.S3Storage{
					Bucket:   "as-backup-bucket",
					Path:     "backups",
					S3Region: "eu-central-1",
				},
			}},
		},
		"routine2": {{
			BackupMetadata: dto.BackupMetadata{
				Created:     time.Date(2024, 01, 01, 12, 0, 0, 0, time.UTC),
				Namespace:   "source-ns2",
				ByteCount:   1_234_567_890,
				RecordCount: 1890,
				FileCount:   4,
			},
			Key: "routine2/backup/1704110400000/source-ns2",
			Storage: &dto.Storage{
				S3Storage: &dto.S3Storage{
					Bucket:   "as-backup-bucket",
					Path:     "backups",
					S3Region: "eu-central-1",
				},
			},
		}},
	},
	"RestoreFullRequest": dto.RestoreRequest{
		DestinationCluster: &dto.AerospikeCluster{
			SeedNodes: []dto.SeedNode{{
				HostName: "host.docker.internal", Port: 3000},
			},
			Credentials: &dto.Credentials{
				User:     util.Ptr("user"),
				Password: util.Ptr("password"),
			},
		},
		Policy: &dto.RestorePolicy{
			NoGeneration: util.Ptr(true),
		},
		SourceStorage: &dto.Storage{
			S3Storage: &dto.S3Storage{
				Bucket:   "as-backup-bucket",
				Path:     "backups",
				S3Region: "eu-central-1",
			},
		},
		BackupDataPath: "routine1/backup/1704110400000/source-ns1",
	},
	"RestoreTimestampRequest": dto.RestoreTimestampRequest{
		DestinationCluster: &dto.AerospikeCluster{
			SeedNodes: []dto.SeedNode{{
				HostName: "host.docker.internal", Port: 3000},
			},
			Credentials: &dto.Credentials{
				User:     util.Ptr("user"),
				Password: util.Ptr("password"),
			},
		},
		Policy: &dto.RestorePolicy{
			NoGeneration: util.Ptr(true),
		},
		Time:    1704110400000,
		Routine: "routine1",
	},
}

func main() {
	_ = dto.AerospikeCluster{}
	readme, err := os.ReadFile("README.md")
	if err != nil {
		panic(err)
	}

	re := regexp.MustCompile("<!--\\s*(\\w+)\\s*-->\\s*```json[\\s\\S]*?```")
	updatedReadme := re.ReplaceAllFunc(readme, func(match []byte) []byte {
		submatches := re.FindSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		name := string(submatches[1])
		example, exists := jsonExamples[name]
		if !exists {
			return match
		}

		formattedJSON, err := json.MarshalIndent(example, "", "  ")
		if err != nil {
			panic(err)
		}

		var buffer bytes.Buffer
		buffer.WriteString(fmt.Sprintf("<!-- %s -->\n```json\n", name))
		buffer.Write(formattedJSON)
		buffer.WriteString("\n```")
		return buffer.Bytes()
	})

	err = os.WriteFile("README.md", updatedReadme, 0644)
	if err != nil {
		panic(err)
	}
}
