package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service"
	"github.com/reugn/go-quartz/quartz"
)

const (
	testDir             = "/testdata"
	testJobID           = 1
	testRoutineName     = "testRoutine"
	testBackupDetailKey = "storage/daily/backup/1707915600000/source-ns1"
)

var errTest = errors.New("test error")

func testRestoreJobStatus() *model.RestoreJobStatus {
	estimatedEndTime := time.Now().Add(1 * time.Minute)
	return &model.RestoreJobStatus{
		RestoreStats: model.RestoreStats{},
		CurrentRestore: &model.RunningJob{
			TotalRecords:     100,
			DoneRecords:      10,
			StartTime:        time.Now(),
			PercentageDone:   10,
			EstimatedEndTime: &estimatedEndTime,
		},
		Status: "Running",
	}
}

func testBackupDetails() model.BackupDetails {
	return model.BackupDetails{
		BackupMetadata: model.BackupMetadata{
			Created:             time.Now(),
			From:                time.Now().AddDate(0, 0, -1),
			Namespace:           "ns",
			RecordCount:         0,
			ByteCount:           0,
			FileCount:           0,
			SecondaryIndexCount: 0,
			UDFCount:            0,
		},
		Key: testBackupDetailKey,
	}
}

func testConfig() *dto.Config {
	clusters := make(map[string]*dto.AerospikeCluster)
	cluster := testConfigCluster()
	clusters[testCluster] = cluster

	policies := make(map[string]*dto.BackupPolicy)
	policy := testConfigBackupPolicy()
	policies[testPolicy] = policy
	policies[unusedTestPolicy] = policy

	storages := make(map[string]*dto.Storage)
	storage := testConfigStorage()
	storages[testStorage] = storage
	storages[unusedTestStorage] = storage

	routines := make(map[string]*dto.BackupRoutine)
	routines[testRoutineName] = &dto.BackupRoutine{
		Storage:       testStorage,
		BackupPolicy:  testPolicy,
		SourceCluster: testCluster,
		IntervalCron:  "0 0 * * * *",
	}

	return &dto.Config{
		ServiceConfig: dto.BackupServiceConfig{
			HTTPServer: &dto.HTTPServerConfig{},
			Logger:     &dto.LoggerConfig{},
		},
		AerospikeClusters: clusters,
		Storage:           storages,
		BackupPolicies:    policies,
		BackupRoutines:    routines,
	}
}

type restoreManagerMock struct{}

func (mock restoreManagerMock) Restore(request *model.RestoreRequest) (model.RestoreJobID, error) {
	if request.BackupDataPath != testDir {
		return 0, errTest
	}
	return model.RestoreJobID(testJobID), nil
}

func (mock restoreManagerMock) RestoreByTime(request *model.RestoreTimestampRequest) (model.RestoreJobID, error) {
	if request.Time == time.UnixMilli(0) {
		return 0, errTest
	}
	return model.RestoreJobID(testJobID), nil
}

func (mock restoreManagerMock) JobStatus(jobID model.RestoreJobID) (*model.RestoreJobStatus, error) {
	if jobID != model.RestoreJobID(testJobID) {
		return nil, errTest
	}
	return testRestoreJobStatus(), nil
}

func (mock restoreManagerMock) RetrieveConfiguration(routine string, _ time.Time) ([]byte, error) {
	if routine == "" {
		return nil, errTest
	}
	return []byte(fmt.Sprintf(`{ "dir": "%s" }`, testDir)), nil
}

type backupListReaderMock struct{}

func (mock backupListReaderMock) FullBackupList(_ context.Context, timebounds *model.TimeBounds,
) ([]model.BackupDetails, error) {
	if timebounds == nil {
		return nil, errTest
	}
	return []model.BackupDetails{testBackupDetails()}, nil
}
func (mock backupListReaderMock) IncrementalBackupList(_ context.Context, timebounds *model.TimeBounds,
) ([]model.BackupDetails, error) {
	if timebounds == nil {
		return nil, errTest
	}
	return []model.BackupDetails{testBackupDetails()}, nil
}

func (mock backupListReaderMock) ReadClusterConfiguration(path string) ([]byte, error) {
	if path == "" {
		return nil, errTest
	}
	return []byte(fmt.Sprintf(`{ "dir": "%s" }`, testDir)), nil
}

func (mock backupListReaderMock) FindLastFullBackup(_ time.Time) ([]model.BackupDetails, error) {
	return []model.BackupDetails{testBackupDetails()}, nil
}

func (mock backupListReaderMock) FindIncrementalBackupsForNamespace(
	_ context.Context, bounds *model.TimeBounds, _ string,
) ([]model.BackupDetails, error) {
	if bounds == nil {
		return nil, errTest
	}
	return []model.BackupDetails{testBackupDetails()}, nil
}

type backendsHolderMock struct{}

func (mock backendsHolderMock) GetReader(routineName string) (service.BackupListReader, bool) {
	if routineName != testRoutineName {
		return nil, false
	}
	return backupListReaderMock{}, true
}

func (mock backendsHolderMock) Get(routineName string) (*service.BackupBackend, bool) {
	if routineName != testRoutineName {
		return nil, false
	}
	// We can't mock this entity, as it StorageAccessor with private methods.
	return &service.BackupBackend{}, true
}

func (mock backendsHolderMock) SetData(_ map[string]*service.BackupBackend) {
}

func (mock backendsHolderMock) GetAllReaders() map[string]service.BackupListReader {
	return nil
}

type configurationManagerMock struct{}

func (mock configurationManagerMock) Read(_ context.Context) (*model.Config, error) {
	return testConfig().ToModel()
}

func (mock configurationManagerMock) Update(_ context.Context, _ func(*model.Config) error) error {
	return nil
}

func (mock configurationManagerMock) Write(_ context.Context, config *model.Config) error {
	if config == nil {
		return errTest
	}
	return nil
}

func newServiceMock() *Service {
	toModel, _ := testConfig().ToModel()
	return &Service{
		config:               toModel,
		scheduler:            quartz.NewStdScheduler(),
		restoreManager:       restoreManagerMock{},
		backupBackends:       backendsHolderMock{},
		handlerHolder:        nil,
		configurationManager: configurationManagerMock{},
		logger:               slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
}
