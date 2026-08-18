package service

import (
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestClusterConfigWriter_Write_GetClientError(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := &model.BackupRoutine{Name: "routine-1", SourceCluster: &model.AerospikeCluster{}}
	clientManager := aerospike.NewMockClientManager(ctrl)
	clientErr := errors.New("connection failed")
	clientManager.EXPECT().
		GetClient(gomock.Any(), routine.SourceCluster, nil, gomock.Any()).
		Return(nil, clientErr)

	writer := NewClusterConfigWriter(clientManager, NewPathService(nil), nil)
	err := writer.Write(t.Context(), routine, time.Now())

	require.Error(t, err)
	require.ErrorIs(t, err, clientErr)
}
