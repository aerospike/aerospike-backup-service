package aerospike

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestClusterConfigSource_NodeConfigs(t *testing.T) {
	clusterCfg := &model.AerospikeCluster{}
	clientErr := errors.New("connection failed")
	configs := []asconfig.DotConf{"namespace ns1 {}", "namespace ns2 {}"}
	logger := slog.New(slog.DiscardHandler)

	tests := []struct {
		name    string
		setup   func(ctrl *gomock.Controller) ClusterConfigSource
		want    []asconfig.DotConf
		wantErr error
	}{
		{
			name: "returns node configs",
			setup: func(ctrl *gomock.Controller) ClusterConfigSource {
				asClient, _, manager := expectClientLifecycle(ctrl, clusterCfg, logger)
				return &clusterConfigSource{
					clientManager: manager,
					readConfiguration: func(got Cluster, _ *slog.Logger) []asconfig.DotConf {
						require.Equal(t, asClient, got)
						return configs
					},
				}
			},
			want: configs,
		},
		{
			name: "get client error",
			setup: func(ctrl *gomock.Controller) ClusterConfigSource {
				manager := NewMockClientManager(ctrl)
				manager.EXPECT().
					GetClient(gomock.Any(), clusterCfg, nil, logger).
					Return(nil, clientErr)

				return NewClusterConfigSource(manager)
			},
			wantErr: clientErr,
		},
		{
			name: "no configuration",
			setup: func(ctrl *gomock.Controller) ClusterConfigSource {
				asClient, _, manager := expectClientLifecycle(ctrl, clusterCfg, logger)
				asClient.EXPECT().Cluster().Return(&as.Cluster{})

				return NewClusterConfigSource(manager)
			},
			wantErr: errors.New("failed to read Aerospike configuration"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			got, err := tc.setup(ctrl).NodeConfigs(t.Context(), clusterCfg, logger)

			if tc.wantErr != nil {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.wantErr.Error())
				require.Nil(t, got)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestGetActiveHosts(t *testing.T) {
	host1 := as.NewHost("127.0.0.1", 3000)
	host2 := as.NewHost("127.0.0.1", 3001)

	tests := []struct {
		name  string
		nodes []clusterNode
		want  []*as.Host
	}{
		{
			name:  "active nodes",
			nodes: []clusterNode{fakeNode{active: true, host: host1}, fakeNode{active: true, host: host2}},
			want:  []*as.Host{host1, host2},
		},
		{
			name:  "skips inactive nodes",
			nodes: []clusterNode{fakeNode{active: false, host: host1}, fakeNode{active: true, host: host2}},
			want:  []*as.Host{host2},
		},
		{
			name:  "all inactive",
			nodes: []clusterNode{fakeNode{active: false, host: host1}},
		},
		{
			name: "empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, getActiveHosts(tc.nodes))
		})
	}
}

func expectClientLifecycle(
	ctrl *gomock.Controller,
	clusterCfg *model.AerospikeCluster,
	logger *slog.Logger,
) (*MockAerospikeClient, *MockClient, *MockClientManager) {
	asClient := NewMockAerospikeClient(ctrl)
	client := NewMockClient(ctrl)
	client.EXPECT().AerospikeClient().Return(asClient)

	manager := NewMockClientManager(ctrl)
	manager.EXPECT().
		GetClient(gomock.Any(), clusterCfg, nil, logger).
		Return(client, nil)
	manager.EXPECT().Close(client)

	return asClient, client, manager
}

type fakeNode struct {
	active bool
	host   *as.Host
}

func (n fakeNode) IsActive() bool    { return n.active }
func (n fakeNode) GetHost() *as.Host { return n.host }
