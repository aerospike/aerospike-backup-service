package main

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// The worked examples the documentation shows, built from the DTO structs so
// that an example can never describe a shape the service would reject. Each key
// is a tag id: <!-- tag RestoreFullRequest --> renders the entry below it.
//
// They live apart from the generator because they are content, not machinery —
// reviewing a change to an example should not mean reading past the engine that
// renders it.
const (
	valLocal          = "local"
	valBackups        = "backups"
	valAsBackupBucket = "as-backup-bucket"
	valEuCentral1     = "eu-central-1"
	valRoutine1       = "routine1"
)

var allStorageTypes = map[string]dto.Storage{
	valLocal: {
		LocalStorage: &dto.LocalStorage{
			Path: valBackups,
		},
	},
	"aws-s3": {
		S3Storage: &dto.S3Storage{
			Bucket:   valAsBackupBucket,
			Path:     valBackups,
			S3Region: valEuCentral1,
		},
	},
	"gcp-gcs": {
		GcpStorage: &dto.GcpStorage{
			Path:       valBackups,
			KeyFile:    "key-file.json",
			BucketName: "gcp-backup-bucket",
			Endpoint:   "http://127.0.0.1:9020",
		},
	},
	"azure-blob-storage": {
		AzureStorage: &dto.AzureStorage{
			Path:          valBackups,
			Endpoint:      "http://127.0.0.1:6000/devstoreaccount1",
			AccountName:   "devstoreaccount1",
			ContainerName: "testcontainer",
			AccountKey:    "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
		},
	},
}

var cluster = dto.AerospikeCluster{
	SeedNodes: []dto.SeedNode{{
		HostName: "host.docker.internal", Port: 3000},
	},
	Credentials: &dto.Credentials{
		User:     "user",
		Password: "password",
	},
}

var jsonExamples = map[string]any{
	"ClustersResponse": []dto.AerospikeCluster{cluster},
	"RoutinesResponse": map[string]dto.BackupRoutine{
		valRoutine1: {
			BackupPolicy:  "keepFilesPolicy",
			SourceCluster: "absDefaultCluster",
			Storage:       valLocal,
			IntervalCron:  "@yearly",
			Namespaces:    ptr.Of([]string{"test-namespace"}),
		},
		"routine2": {
			BackupPolicy:     "removeFilesPolicy",
			SourceCluster:    "absDefaultCluster",
			Storage:          valLocal,
			IntervalCron:     "@monthly",
			IncrIntervalCron: "@daily",
			Namespaces:       ptr.Of([]string{"test-namespace"}),
			SetList:          []string{"backupSet"},
			BinList:          []string{"backupBin"},
		},
	},
	"StorageResponse": allStorageTypes,
	"FullBackupsResponse": map[string][]dto.BackupDetails{
		valRoutine1: {{
			Created:             time.Date(2024, 01, 01, 12, 0, 0, 0, time.UTC),
			Timestamp:           time.Date(2024, 01, 01, 12, 0, 0, 0, time.UTC).UnixMilli(),
			Finished:            time.Date(2024, 01, 01, 12, 5, 0, 0, time.UTC),
			Duration:            300,
			From:                time.Time{},
			Namespace:           "source-ns1",
			RecordCount:         42,
			ByteCount:           480_000,
			FileCount:           1,
			SecondaryIndexCount: 5,
			UDFCount:            1,
			Key:                 "routine1/backup/1704110400000/source-ns1",
			Storage: &dto.Storage{
				S3Storage: &dto.S3Storage{
					Bucket:   valAsBackupBucket,
					Path:     valBackups,
					S3Region: valEuCentral1,
				},
			},
			Compression: string(dto.CompressionModeZSTD),
			Encryption:  string(dto.EncryptionModeNone),
		},
		},
	},
	"RestoreFullRequest": dto.RestoreRequest{
		DestinationClusterConfig: dto.DestinationClusterConfig{
			Cluster: &cluster,
		},
		Policy: &dto.RestorePolicy{
			BaseRestorePolicy: dto.BaseRestorePolicy{
				NoGeneration: ptr.Of(true),
			},
		},
		StorageConfig: dto.StorageConfig{
			Storage: &dto.Storage{
				S3Storage: &dto.S3Storage{
					Bucket:   valAsBackupBucket,
					Path:     valBackups,
					S3Region: valEuCentral1,
				},
			},
		},
		BackupDataPath: "routine1/backup/1704110400000/source-ns1",
	},
	"RestoreTimestampRequest": dto.RestoreTimestampRequest{
		DestinationClusterConfig: dto.DestinationClusterConfig{
			Name: "abs-cluster",
		},
		Time:    1704110400000,
		Routine: valRoutine1,
	},
	"CurrentBackupResponse": dto.RoutineState{
		Full: &dto.RunningJob{
			TotalRecords:     100_000,
			DoneRecords:      50_000,
			StartTime:        time.Date(2024, 01, 01, 12, 0, 0, 0, time.UTC),
			FinishTime:       nil,
			PercentageDone:   50,
			EstimatedEndTime: ptr.Of(time.Date(2024, 01, 01, 13, 0, 0, 0, time.UTC)),
			Duration:         1800,
			Metrics: &dto.Metrics{
				RecordsPerSecond:   1000,
				KilobytesPerSecond: 30000,
				Pipeline:           167,
			},
		},
	},
	"CurrentRestoreResponse": dto.RestoreJobStatus{
		ReadRecords:     100_000,
		TotalBytes:      30000000,
		ExpiredRecords:  0,
		SkippedRecords:  0,
		IgnoredRecords:  0,
		InsertedRecords: 50_000,
		ExistedRecords:  0,
		FresherRecords:  0,
		IndexCount:      4,
		UDFCount:        1,
		ErrorsInDoubt:   0,
		CurrentRestore: &dto.RunningJob{
			TotalRecords:     100_000,
			DoneRecords:      50_000,
			StartTime:        time.Date(2024, 01, 01, 12, 0, 0, 0, time.UTC),
			FinishTime:       nil,
			PercentageDone:   50,
			EstimatedEndTime: ptr.Of(time.Date(2024, 01, 01, 13, 0, 0, 0, time.UTC)),
			Duration:         1800,
			Metrics: &dto.Metrics{
				RecordsPerSecond:   1000,
				KilobytesPerSecond: 30000,
				Pipeline:           8192,
			},
		},
		Status: dto.RestoreRunning,
		Error:  "",
	},
	"CurrentRestoresResponse": map[int]dto.RestoreJobStatus{
		12345678: {
			ReadRecords:     100_000,
			TotalBytes:      30000000,
			ExpiredRecords:  0,
			SkippedRecords:  0,
			IgnoredRecords:  0,
			InsertedRecords: 50_000,
			ExistedRecords:  0,
			FresherRecords:  0,
			IndexCount:      4,
			UDFCount:        1,
			ErrorsInDoubt:   0,
			CurrentRestore: &dto.RunningJob{
				TotalRecords:     100_000,
				DoneRecords:      50_000,
				StartTime:        time.Date(2024, 01, 01, 12, 0, 0, 0, time.UTC),
				FinishTime:       nil,
				PercentageDone:   50,
				Duration:         1800,
				EstimatedEndTime: ptr.Of(time.Date(2024, 01, 01, 13, 0, 0, 0, time.UTC)),
				Metrics: &dto.Metrics{
					RecordsPerSecond:   1000,
					KilobytesPerSecond: 30000,
					Pipeline:           0,
				},
			},
			Status: dto.RestoreRunning,
			Error:  "",
		}},
}

var yamlExamples = map[string]any{
	"Storage": allStorageTypes,
	"RemoteConfig": dto.Storage{
		S3Storage: &dto.S3Storage{
			Path:     "config.yml",
			Bucket:   valAsBackupBucket,
			S3Region: valEuCentral1,
		},
	},
}
