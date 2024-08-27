package handlers

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/internal/server/dto"
	"github.com/aerospike/aerospike-backup-service/pkg/service"
	"github.com/reugn/go-quartz/quartz"
)

const (
	testDir             = "/testdata"
	testJobID           = 1
	testRoutineName     = "testRoutine"
	testBackupDetailKey = "storage/daily/backup/1707915600000/source-ns1"
)

var errTest = errors.New("test error")

func testRestoreJobStatus() *dto.RestoreJobStatus {
	estimatedEndTime := time.Now().Add(1 * time.Minute)
	return &dto.RestoreJobStatus{
		RestoreStats: dto.RestoreStats{},
		CurrentRestore: &dto.RunningJob{
			TotalRecords:     100,
			DoneRecords:      10,
			StartTime:        time.Now(),
			PercentageDone:   10,
			EstimatedEndTime: &estimatedEndTime,
		},
		Status: "Running",
	}
}

func testBackupDetails() dto.BackupDetails {
	key := testBackupDetailKey
	return dto.BackupDetails{
		BackupMetadata: dto.BackupMetadata{
			Created:             time.Now(),
			From:                time.Now().AddDate(0, 0, -1),
			Namespace:           "ns",
			RecordCount:         0,
			ByteCount:           0,
			FileCount:           0,
			SecondaryIndexCount: 0,
			UDFCount:            0,
		},
		Key: &key,
	}
}

func testConfig() *dto.Config {
	clusters := make(map[string]*dto.AerospikeCluster)
	cluster := testConfigCluster()
	clusters[testCluster] = &cluster

	policies := make(map[string]*dto.BackupPolicy)
	policy := testConfigBackupPolicy()
	policies[testPolicy] = &policy

	storages := make(map[string]*dto.Storage)
	storage := testConfigStorage()
	storages[testStorage] = &storage

	routines := make(map[string]*dto.BackupRoutine)
	routines[testRoutineName] = &dto.BackupRoutine{
		IntervalCron: "0 0 * * * *",
	}

	return &dto.Config{
		ServiceConfig: &dto.BackupServiceConfig{
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

func (mock restoreManagerMock) Restore(request *dto.RestoreRequestInternal) (dto.RestoreJobID, error) {
	if *request.Dir != testDir {
		return 0, errTest
	}
	return dto.RestoreJobID(testJobID), nil
}

func (mock restoreManagerMock) RestoreByTime(request *dto.RestoreTimestampRequest) (dto.RestoreJobID, error) {
	if request.Time == 0 {
		return 0, errTest
	}
	return dto.RestoreJobID(testJobID), nil
}

func (mock restoreManagerMock) JobStatus(jobID dto.RestoreJobID) (*dto.RestoreJobStatus, error) {
	if jobID != dto.RestoreJobID(testJobID) {
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

func (mock backupListReaderMock) FullBackupList(timebounds *dto.TimeBounds) ([]dto.BackupDetails, error) {
	if timebounds == nil {
		return nil, errTest
	}
	return []dto.BackupDetails{testBackupDetails()}, nil
}
func (mock backupListReaderMock) IncrementalBackupList(timebounds *dto.TimeBounds) ([]dto.BackupDetails, error) {
	if timebounds == nil {
		return nil, errTest
	}
	return []dto.BackupDetails{testBackupDetails()}, nil
}

func (mock backupListReaderMock) ReadClusterConfiguration(path string) ([]byte, error) {
	if path == "" {
		return nil, errTest
	}
	return []byte(fmt.Sprintf(`{ "dir": "%s" }`, testDir)), nil
}

func (mock backupListReaderMock) FindLastFullBackup(_ time.Time) ([]dto.BackupDetails, error) {
	return []dto.BackupDetails{testBackupDetails()}, nil
}

func (mock backupListReaderMock) FindIncrementalBackupsForNamespace(bounds *dto.TimeBounds, _ string,
) ([]dto.BackupDetails, error) {
	if bounds == nil {
		return nil, errTest
	}
	return []dto.BackupDetails{testBackupDetails()}, nil
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

type configurationManagerMock struct{}

func (mock configurationManagerMock) ReadConfiguration() (*dto.Config, error) {
	return testConfig(), nil
}

func (mock configurationManagerMock) WriteConfiguration(config *dto.Config) error {
	if config == nil {
		return errTest
	}
	return nil
}

func newServiceMock() *Service {
	return &Service{
		config:               testConfig(),
		scheduler:            quartz.NewStdScheduler(),
		restoreManager:       restoreManagerMock{},
		backupBackends:       backendsHolderMock{},
		handlerHolder:        nil,
		configurationManager: configurationManagerMock{},
		logger:               slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
}
