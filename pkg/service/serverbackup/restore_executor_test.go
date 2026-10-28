package serverbackup

import (
	"context"
	"fmt"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	fullID  = "261020T090111-aaaa"
	incr1ID = "261020T091111-bbbb"
	incr2ID = "261020T092111-cccc"
)

func TestRestoreExecutor_Run_SingleBackupIsFuzzy(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *infomodels.RequestRestore) error {
			assert.Equal(t, fullID, request.BackupIDs)
			assert.Equal(t, "bar", request.Namespace)
			assert.Equal(t, infomodels.RestorePolicyFuzzy, request.Policy)

			return nil
		})

	handler, err := NewRestoreExecutor(namespaceReader(t, ctrl)).
		Run(t.Context(), clientOf(ctrl, infoClient), chainRequest(fullID))
	require.NoError(t, err)
	assert.NotNil(t, handler)
}

func TestRestoreExecutor_Run_ChainIsStrict(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	var preparedJobID string
	infoClient := aerospike.NewMockInfoGetter(ctrl)
	gomock.InOrder(
		infoClient.EXPECT().PrepareRestore(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, request *infomodels.RequestPrepareRestore) error {
				assert.Equal(t, "bar", request.Namespace)
				preparedJobID = request.JobID

				return nil
			}),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", gomock.Any()).Return(
			&infomodels.ResponseRestoreState{
				NsState: infomodels.NamespaceStatePreparing,
				Clear:   infomodels.ClearStateRequired,
			}, nil),
		infoClient.EXPECT().ClearRestoreStorage(gomock.Any(), "bar", gomock.Any()).Return(nil),
		infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", gomock.Any()).Return(
			&infomodels.ResponseRestoreState{NsState: infomodels.NamespaceStateReady}, nil),
		infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, request *infomodels.RequestRestore) error {
				assert.Equal(t, fullID+","+incr1ID+","+incr2ID, request.BackupIDs)
				assert.Equal(t, "bar", request.Namespace)
				assert.Equal(t, infomodels.RestorePolicyStrict, request.Policy)
				assert.Equal(t, preparedJobID, request.JobID, "the restore starts the prepared job")

				return nil
			}),
	)

	handler, err := NewRestoreExecutor(namespaceReader(t, ctrl)).Run(t.Context(), clientOf(ctrl, infoClient),
		chainRequest(fullID), chainRequest(incr1ID), chainRequest(incr2ID))
	require.NoError(t, err)
	assert.NotNil(t, handler)
}

func TestRestoreExecutor_Run_ChainPreparationFails(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().PrepareRestore(gomock.Any(), gomock.Any()).Return(nil)
	infoClient.EXPECT().GetRestoreStatus(gomock.Any(), "bar", gomock.Any()).Return(
		&infomodels.ResponseRestoreState{NsState: infomodels.NamespaceStateFailed, ErrorReason: "not SSD"}, nil)

	_, err := NewRestoreExecutor(namespaceReader(t, ctrl)).Run(t.Context(), clientOf(ctrl, infoClient),
		chainRequest(fullID), chainRequest(incr1ID))
	require.ErrorContains(t, err, "not SSD")
}

func TestRestoreExecutor_Run_ChainTooLong(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	requests := make([]*model.RestoreRequest, maxChainLength+1)
	for i := range requests {
		requests[i] = chainRequest(fmt.Sprintf("261020T09%04d-aaaa", i))
	}

	_, err := NewRestoreExecutor(service.NewMockBackupReader(ctrl)).
		Run(t.Context(), aerospike.NewMockClient(ctrl), requests[0], requests[1:]...)
	require.ErrorContains(t, err, "at most 32 backups")
}

// namespaceReader returns a BackupReader that reports the full backup in namespace bar.
func namespaceReader(t *testing.T, ctrl *gomock.Controller) service.BackupReader {
	t.Helper()

	backupReader := service.NewMockBackupReader(ctrl)
	backupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, filter service.BackupFilter) ([]model.BackupDetails, error) {
			assert.Equal(t, fullID, filter.(*service.PathFilter).Path(), "the namespace comes from the full backup")

			return []model.BackupDetails{{BackupMetadata: model.BackupMetadata{Namespace: "bar"}}}, nil
		})

	return backupReader
}

func clientOf(ctrl *gomock.Controller, infoClient *aerospike.MockInfoGetter) aerospike.Client {
	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().InfoClient().Return(infoClient)

	return client
}

func chainRequest(backupID string) *model.RestoreRequest {
	return &model.RestoreRequest{
		SourceStorage:  &model.S3Storage{Bucket: "bucket", S3Region: "us-east-1"},
		BackupDataPath: backupID,
	}
}
