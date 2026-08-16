package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// fakeStorageDataWriter is a controllable stand-in for storageDataWriter.
type fakeStorageDataWriter struct {
	writes map[string][]byte
	err    error
}

func (f *fakeStorageDataWriter) WriteDataFile(
	_ context.Context, _ model.Storage, fileName string, content []byte,
) error {
	if f.err != nil {
		return f.err
	}
	if f.writes == nil {
		f.writes = make(map[string][]byte)
	}
	f.writes[fileName] = content
	return nil
}

func TestClusterConfigWriter_Write_GetClientError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := &model.BackupRoutine{Name: "routine-1", SourceCluster: &model.AerospikeCluster{}}
	clientManager := aerospike.NewMockClientManager(ctrl)
	clientErr := errors.New("connection failed")
	clientManager.EXPECT().
		GetClient(gomock.Any(), routine.SourceCluster, nil, gomock.Any()).
		Return(nil, clientErr)

	writer := NewClusterConfigWriter(clientManager, NewPathService(nil), &fakeStorageDataWriter{})
	err := writer.Write(t.Context(), routine, time.Now())

	require.Error(t, err)
	require.ErrorIs(t, err, clientErr)
}

// Note: the success/no-active-hosts paths of Write delegate to the package-level
// cluster.ReadConfiguration, which requires a fully initialized *as.Cluster (real or
// dialed) and isn't mockable at this boundary; those paths are covered by integration
// tests against a live Aerospike cluster instead (see pkg/service/aerospike/cluster/).
