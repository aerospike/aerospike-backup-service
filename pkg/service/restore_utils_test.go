package service

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func waitForRestore(
	t *testing.T,
	restoreManager RestoreManager,
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

// oneByOneRestore passes the backups of a restore to restore one at a time, in the order the
// time runner gives them, so that tests set their expectations per backup.
type oneByOneRestore struct {
	restore *restoreexecutor.MockRestore
}

// Run restores each request after the previous one has finished. The handler of the last one
// started is returned: it reports a failure that ended the chain early from its Wait.
func (o oneByOneRestore) Run(
	ctx context.Context,
	client aerospike.Client,
	full *model.RestoreRequest,
	incrementals ...*model.RestoreRequest,
) (restoreexecutor.RestoreHandler, error) {
	requests := append([]*model.RestoreRequest{full}, incrementals...)

	var handler restoreexecutor.RestoreHandler
	for i, request := range requests {
		started, err := o.restore.Run(ctx, client, request)
		if err != nil {
			return nil, err
		}

		handler = started
		if i < len(requests)-1 && handler.Wait(ctx) != nil {
			break
		}
	}

	return handler, nil //nolint:nilerr // the handler of a failed restore reports it from Wait.
}

// testRestoreEnv holds the components needed for restore tests.
type testRestoreEnv struct {
	ctrl              *gomock.Controller
	mockRestore       *restoreexecutor.MockRestore
	mockClientManager *aerospike.MockClientManager
	infoGetter        *aerospike.MockInfoGetter
	mockBackupReader  *MockBackupReader
	jobsHolder        *RestoreJobsHolder
	restoreValidator  *MockRestoreValidator
	restoreManager    RestoreManager
}

// setupTestRestoreEnv initializes the test environment.
func setupTestRestoreEnv(t *testing.T) *testRestoreEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	infoGetter := aerospike.NewMockInfoGetter(ctrl)

	mockRestore := restoreexecutor.NewMockRestore(ctrl)
	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupReader := NewMockBackupReader(ctrl)
	restoreJobsHolder := NewRestoreJobsHolder()
	restoreJobsHolder.Start(t.Context())
	validator := NewMockRestoreValidator(ctrl)

	restoreManager := NewRestoreManager(
		oneByOneRestore{restore: mockRestore},
		mockClientManager,
		restoreJobsHolder,
		mockBackupReader,
		&collections.LockMap{},
		validator,
	)

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

// expectDefaultRestoreHandler creates a MockRestoreHandler with default EXPECTs:
// Wait returns nil, GetStats returns empty stats, GetMetrics returns empty metrics.
// Returns the handler for use in mockRestore.EXPECT().Run(..., handler, ...).
func (env *testRestoreEnv) expectDefaultRestoreHandler() *restoreexecutor.MockRestoreHandler {
	mockRestoreHandler := restoreexecutor.NewMockRestoreHandler(env.ctrl)
	mockRestoreHandler.EXPECT().Wait(gomock.Any()).Return(nil).AnyTimes()
	mockRestoreHandler.EXPECT().GetStats().Return(models.NewRestoreStats()).AnyTimes()
	mockRestoreHandler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()
	return mockRestoreHandler
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
