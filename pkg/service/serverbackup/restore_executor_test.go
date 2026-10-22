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

func TestRestoreExecutor_Run_ChainInOneJob(t *testing.T) {
	ctrl := gomock.NewController(t)

	backupReader := service.NewMockBackupReader(ctrl)
	backupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, filter service.BackupFilter) ([]model.BackupDetails, error) {
			assert.Equal(t, fullID, filter.(*service.PathFilter).Path(), "the namespace comes from the full backup")

			return []model.BackupDetails{{BackupMetadata: model.BackupMetadata{Namespace: "bar"}}}, nil
		})

	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().StartRestore(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *infomodels.RequestRestore) error {
			assert.Equal(t, fullID+","+incr1ID+","+incr2ID, request.BackupIDs)
			assert.Equal(t, "bar", request.Namespace)
			assert.NotNil(t, request.Fuzzy)

			return nil
		})

	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().InfoClient().Return(infoClient)

	handler, err := NewRestoreExecutor(backupReader).Run(t.Context(), client,
		chainRequest(fullID), chainRequest(incr1ID), chainRequest(incr2ID))
	require.NoError(t, err)
	assert.NotNil(t, handler)
}

func TestRestoreExecutor_Run_ChainTooLong(t *testing.T) {
	ctrl := gomock.NewController(t)

	requests := make([]*model.RestoreRequest, maxChainLength+1)
	for i := range requests {
		requests[i] = chainRequest(fmt.Sprintf("261020T09%04d-aaaa", i))
	}

	_, err := NewRestoreExecutor(service.NewMockBackupReader(ctrl)).
		Run(t.Context(), aerospike.NewMockClient(ctrl), requests[0], requests[1:]...)
	require.ErrorContains(t, err, "at most 32 backups")
}

func chainRequest(backupID string) *model.RestoreRequest {
	return &model.RestoreRequest{
		SourceStorage:  &model.S3Storage{Bucket: "bucket", S3Region: "us-east-1"},
		BackupDataPath: backupID,
	}
}
