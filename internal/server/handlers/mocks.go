package handlers

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/reugn/go-quartz/quartz"
	"github.com/stretchr/testify/mock"
)

// MockBackupBackendService is a mock implementation of BackupReaderWriter.
type MockBackupBackendService struct {
	mock.Mock
}

// GetBackups retrieves backup details based on the provided filter.
func (m *MockBackupBackendService) GetBackups(ctx context.Context, filter service.BackupFilter,
) ([]model.BackupDetails, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]model.BackupDetails), args.Error(1)
}

// WriteBackupMetadata stores metadata for a specific backup.
func (m *MockBackupBackendService) WriteBackupMetadata(
	ctx context.Context,
	routineName, path string,
	metadata model.BackupMetadata,
) error {
	args := m.Called(ctx, routineName, path, metadata)
	return args.Error(0)
}

// Delete removes a specific backup folder.
func (m *MockBackupBackendService) Delete(ctx context.Context, routineName *model.BackupRoutine, path string) error {
	args := m.Called(ctx, routineName, path)
	return args.Error(0)
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

func (a *MockConfigApplier) ApplyNewConfig(_ context.Context) error {
	return nil
}
