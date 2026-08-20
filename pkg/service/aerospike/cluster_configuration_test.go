package aerospike

import (
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestClusterConfigSource_NodeConfigs_GetClientError(t *testing.T) {
	ctrl := gomock.NewController(t)

	clusterCfg := &model.AerospikeCluster{}
	clientErr := errors.New("connection failed")
	clientManager := NewMockClientManager(ctrl)
	clientManager.EXPECT().
		GetClient(gomock.Any(), clusterCfg, nil, gomock.Any()).
		Return(nil, clientErr)

	source := NewClusterConfigSource(clientManager)
	_, err := source.NodeConfigs(t.Context(), clusterCfg, nil)

	require.Error(t, err)
	require.ErrorIs(t, err, clientErr)
}
