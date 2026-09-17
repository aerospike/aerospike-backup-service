package aerospike

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// A client that fails its health check is closed and forgotten, so the next GetClient opens a
// fresh connection.
func Test_GetClient_ReconnectsAfterFailedHealthCheck(t *testing.T) {
	ctrl := gomock.NewController(t)

	factory := NewMockClientFactory(ctrl)
	asClient := NewMockAerospikeClient(ctrl)
	backupClient := NewMockClient(ctrl)
	info := NewMockInfoGetter(ctrl)

	info.EXPECT().GetStatus(gomock.Any()).Return("fail", nil).Times(2)
	backupClient.EXPECT().InfoClient().Return(info).Times(2)
	factory.EXPECT().NewBackupClient(gomock.Any(), gomock.Any()).Return(backupClient, nil).Times(2)
	// Every failed health check drops the connection, so every attempt connects anew.
	factory.EXPECT().NewClientWithPolicyAndHost(gomock.Any(), gomock.Any()).Return(asClient, nil).Times(2)
	asClient.EXPECT().Close().Times(2)

	cm := NewClientManager(factory, time.Millisecond)
	c := &model.AerospikeCluster{ClusterLabel: "dead"}

	for range 2 {
		_, err := cm.GetClient(t.Context(), c, nil, nil)
		require.Error(t, err)
	}

	_, cached := cm.(*clientManager).clients.Load(c.Hash())
	assert.False(t, cached, "dead client must not stay cached")
}
