package handlers

import (
	"context"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/reugn/go-quartz/quartz"
	"github.com/stretchr/testify/mock"
)

// MockBackendsHolder mocks the BackendsHolder interface.
type MockBackendsHolder struct {
	mock.Mock
}

func (m *MockBackendsHolder) Init(routines map[string]*model.BackupRoutine) {
	m.Called(routines)
}

func (m *MockBackendsHolder) GetReader(routineName string) (service.BackupMetadataReader, bool) {
	args := m.Called(routineName)
	return args.Get(0).(service.BackupMetadataReader), args.Bool(1)
}

func (m *MockBackendsHolder) Get(routineName string) (service.BackupMetadataReaderWriter, bool) {
	args := m.Called(routineName)
	return args.Get(0).(service.BackupMetadataReaderWriter), args.Bool(1)
}

func (m *MockBackendsHolder) GetAllReaders() map[string]service.BackupMetadataReader {
	args := m.Called()
	return args.Get(0).(map[string]service.BackupMetadataReader)
}

// MockBackupMetadataReader mocks the BackupMetadataReader interface.
type MockBackupMetadataReader struct {
	mock.Mock
}

func (m *MockBackupMetadataReader) FullBackupList(ctx context.Context, timeBounds model.TimeBounds,
) ([]model.BackupDetails, error) {
	args := m.Called(ctx, timeBounds)
	return args.Get(0).([]model.BackupDetails), args.Error(1)
}

func (m *MockBackupMetadataReader) IncrementalBackupList(ctx context.Context, timeBounds model.TimeBounds,
) ([]model.BackupDetails, error) {
	args := m.Called(ctx, timeBounds)
	return args.Get(0).([]model.BackupDetails), args.Error(1)
}

func (m *MockBackupMetadataReader) ReadClusterConfiguration(path string) ([]byte, error) {
	args := m.Called(path)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockBackupMetadataReader) FindLastFullBackup(toTime time.Time) ([]model.BackupDetails, error) {
	args := m.Called(toTime)
	return args.Get(0).([]model.BackupDetails), args.Error(1)
}

func (m *MockBackupMetadataReader) FindIncrementalBackupsForNamespace(
	ctx context.Context, bounds model.TimeBounds, namespace string,
) ([]model.BackupDetails, error) {
	args := m.Called(ctx, bounds, namespace)
	return args.Get(0).([]model.BackupDetails), args.Error(1)
}

func (m *MockBackupMetadataReader) FindLastRun(ctx context.Context) *model.LastBackupRun {
	args := m.Called(ctx)
	return args.Get(0).(*model.LastBackupRun)
}

// MockScheduler mocks the quartz.Scheduler interface.
type MockScheduler struct {
	mock.Mock
}

func (m *MockScheduler) Start(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockScheduler) IsStarted() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockScheduler) ScheduleJob(jobDetail *quartz.JobDetail, trigger quartz.Trigger) error {
	args := m.Called(jobDetail, trigger)
	return args.Error(0)
}

func (m *MockScheduler) GetJobKeys(matchers ...quartz.Matcher[quartz.ScheduledJob]) ([]*quartz.JobKey, error) {
	args := m.Called(matchers)
	return args.Get(0).([]*quartz.JobKey), args.Error(1)
}

func (m *MockScheduler) GetScheduledJob(jobKey *quartz.JobKey) (quartz.ScheduledJob, error) {
	args := m.Called(jobKey)
	return args.Get(0).(quartz.ScheduledJob), args.Error(1)
}

func (m *MockScheduler) DeleteJob(jobKey *quartz.JobKey) error {
	args := m.Called(jobKey)
	return args.Error(0)
}

func (m *MockScheduler) PauseJob(jobKey *quartz.JobKey) error {
	args := m.Called(jobKey)
	return args.Error(0)
}

func (m *MockScheduler) ResumeJob(jobKey *quartz.JobKey) error {
	args := m.Called(jobKey)
	return args.Error(0)
}

func (m *MockScheduler) Clear() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockScheduler) Wait(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockScheduler) Stop() {
	m.Called()
}

// MockRestoreManager mocks the RestoreManager interface.
type MockRestoreManager struct {
	mock.Mock
}

func (m *MockRestoreManager) Restore(request *model.RestoreRequest) (model.RestoreJobID, error) {
	args := m.Called(request)
	return args.Get(0).(model.RestoreJobID), args.Error(1)
}

func (m *MockRestoreManager) RestoreByTime(request *model.RestoreTimestampRequest) (model.RestoreJobID, error) {
	args := m.Called(request)
	return args.Get(0).(model.RestoreJobID), args.Error(1)
}

func (m *MockRestoreManager) JobStatus(jobID model.RestoreJobID) (*model.RestoreJobStatus, error) {
	args := m.Called(jobID)
	return args.Get(0).(*model.RestoreJobStatus), args.Error(1)
}

func (m *MockRestoreManager) RetrieveConfiguration(routine string, toTime time.Time) ([]byte, error) {
	args := m.Called(routine, toTime)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockRestoreManager) CancelRestore(jobID model.RestoreJobID) error {
	args := m.Called(jobID)
	return args.Error(0)
}

type configurationManagerMock struct{}

func (mock configurationManagerMock) Read(_ context.Context) (*model.Config, error) {
	return nil, nil
}

func (mock configurationManagerMock) Write(_ context.Context, config *model.Config) error {
	if config == nil {
		return nil
	}
	return nil
}

type MockConfigApplier struct{}

func (a *MockConfigApplier) ApplyNewRoutines(_ context.Context, _ map[string]*model.BackupRoutine) error {
	return nil
}

type mockNamespaceValidator struct {
	validateError error
}

func (m *mockNamespaceValidator) IsEmpty(_ aerospike.Cluster, _ string, _ []string) (bool, error) {
	return false, nil
}

func (m *mockNamespaceValidator) MissingNamespaces(_ *model.AerospikeCluster, _ []string) []string {
	return nil
}

func (m *mockNamespaceValidator) ValidateRoutines(_ *model.AerospikeCluster, _ map[string]*model.BackupRoutine) error {
	return m.validateError
}

type mockRunningBackupsRegistry struct {
	mock.Mock
}

func (m *mockRunningBackupsRegistry) GetRoutineState(routineName string) *model.RoutineState {
	args := m.Called(routineName)
	if state, ok := args.Get(0).(*model.RoutineState); ok {
		return state
	}
	return nil
}

func (m *mockRunningBackupsRegistry) GetRunningState() map[string]*model.RoutineState {
	args := m.Called()
	if state, ok := args.Get(0).(map[string]*model.RoutineState); ok {
		return state
	}
	return nil
}

func (m *mockRunningBackupsRegistry) Cancel(routineName string) {
	m.Called(routineName)
}
