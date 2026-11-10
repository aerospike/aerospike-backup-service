package scanbackup

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func waitForRestore(
	t *testing.T,
	restoreManager service.RestoreManager,
	jobID model.RestoreJobID,
) (*model.RestoreJobStatus, error) {
	t.Helper()
	var (
		jobStatus *model.RestoreJobStatus
		err       error
	)

	assert.Eventually(t, func() bool {
		jobStatus, err = restoreManager.JobStatus(jobID)
		if err != nil {
			return false
		}
		return jobStatus.Status != model.RestoreRunning
	}, 2*time.Second, 10*time.Millisecond, "Job should eventually stop running")

	return jobStatus, err
}

// failedRestore is the handler of a restore that failed to start.
type failedRestore struct {
	err error
}

func (f failedRestore) Wait(context.Context) error { return f.err }
func (f failedRestore) Status() model.RestoreRun   { return model.RestoreRun{} }

// testRestoreEnv holds the components needed for restore tests.
type testRestoreEnv struct {
	ctrl              *gomock.Controller
	mockRestore       *MockRestoreExecutor
	mockClientManager *aerospike.MockClientManager
	infoGetter        *aerospike.MockInfoGetter
	mockBackupReader  *service.MockBackupReader
	jobsHolder        *service.RestoreJobsHolder
	restoreValidator  *MockRestoreValidator
	restoreManager    service.RestoreManager
}

// setupTestRestoreEnv initializes the test environment.
func setupTestRestoreEnv(t *testing.T) *testRestoreEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	infoGetter := aerospike.NewMockInfoGetter(ctrl)

	mockRestore := NewMockRestoreExecutor(ctrl)
	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupReader := service.NewMockBackupReader(ctrl)
	restoreJobsHolder := service.NewRestoreJobsHolder()
	restoreJobsHolder.Start(t.Context())
	validator := NewMockRestoreValidator(ctrl)

	restoreManager := NewRestoreManager(
		mockRestore,
		mockClientManager,
		restoreJobsHolder,
		mockBackupReader,
		&collections.LockMap{},
		validator,
	)

	// The destination namespace holds records, so a chain keeps the order it is given in.
	infoGetter.EXPECT().GetRecordCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(1), nil).AnyTimes()

	return &testRestoreEnv{
		ctrl:              ctrl,
		mockRestore:       mockRestore,
		mockClientManager: mockClientManager,
		mockBackupReader:  mockBackupReader,
		jobsHolder:        restoreJobsHolder,
		restoreValidator:  validator,
		restoreManager:    restoreManager,
		infoGetter:        infoGetter,
	}
}

// expectDefaultRestoreHandler creates a service.MockRestoreHandler with default EXPECTs:
// Wait returns nil and Status returns an empty run.
// Returns the handler for use in mockRestore.EXPECT().Run(..., handler, ...).
func (env *testRestoreEnv) expectDefaultRestoreHandler() *service.MockRestoreHandler {
	mockRestoreHandler := service.NewMockRestoreHandler(env.ctrl)
	mockRestoreHandler.EXPECT().Wait(gomock.Any()).Return(nil).AnyTimes()
	mockRestoreHandler.EXPECT().Status().Return(model.RestoreRun{}).AnyTimes()
	return mockRestoreHandler
}

// expectRestores expects one successful restore of each backup the matchers describe, in the
// given order.
func (env *testRestoreEnv) expectRestores(client any, backups ...any) {
	calls := make([]any, 0, len(backups))
	for _, backup := range backups {
		calls = append(calls,
			env.mockRestore.EXPECT().Run(gomock.Any(), client, backup).Return(env.expectDefaultRestoreHandler(), nil))
	}

	gomock.InOrder(calls...)
}

// expectSuccessfulClientInteraction sets up the common GetClient/Close mock expectations.
// It returns the mocked client instance for potential further use in the test.
func (env *testRestoreEnv) expectSuccessfulClientInteraction(
	t *testing.T,
) aerospike.Client {
	t.Helper()

	client := aerospike.NewMockClient(env.ctrl)
	client.EXPECT().InfoClient().Return(env.infoGetter).AnyTimes()

	env.mockClientManager.EXPECT().
		GetClient(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(client, nil).
		AnyTimes()

	env.mockClientManager.EXPECT().
		Close(client).
		AnyTimes()

	return client
}
