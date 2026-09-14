package secrets

import (
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func clusterWithTLS(tlsConfig *model.TLS, agent *model.SecretAgent) *model.AerospikeCluster {
	return &model.AerospikeCluster{
		ClusterLabel: "test-cluster",
		Credentials:  &model.Credentials{SecretAgent: agent},
		TLS:          tlsConfig,
	}
}

func TestClusterTLSResolverResolvesThroughTheClusterAgent(t *testing.T) {
	ctrl := gomock.NewController(t)
	agent := &model.SecretAgent{Address: "127.0.0.1"}
	resolver := NewMockResolver(ctrl)
	resolver.EXPECT().
		Resolve(gomock.Any(), agent, "secrets:agent:key").
		Return("real-password", nil)

	cluster := clusterWithTLS(&model.TLS{
		ClientTLS:       model.ClientTLS{CAFile: "ca.pem", Certfile: "cert.pem", Keyfile: "key.pem"},
		Protocols:       "TLSv1.2",
		KeyfilePassword: "secrets:agent:key",
	}, agent)

	result, err := NewClusterTLSResolver(resolver).Resolve(t.Context(), cluster)
	require.NoError(t, err)
	require.Equal(t, "real-password", result.KeyfilePassword)

	// Every other field survives resolution, and the cluster itself is untouched.
	require.Equal(t, "ca.pem", result.CAFile)
	require.Equal(t, "cert.pem", result.Certfile)
	require.Equal(t, "key.pem", result.Keyfile)
	require.Equal(t, "TLSv1.2", result.Protocols)
	require.Equal(t, "secrets:agent:key", cluster.TLS.KeyfilePassword)
}

// TestClusterTLSResolverSkipsResolverWhenNothingToResolve also pins the contract
// client_factory relies on: a cluster whose seed nodes require TLS but that has no
// TLS block resolves to the zero value, which tlsconfig.NewTLSConfig turns into a
// default configuration.
func TestClusterTLSResolverSkipsResolverWhenNothingToResolve(t *testing.T) {
	ctrl := gomock.NewController(t)
	resolver := NewMockResolver(ctrl) // no EXPECT(): Resolve must not be called

	withCAFile := model.TLS{ClientTLS: model.ClientTLS{CAFile: "ca.pem"}}

	tests := map[string]struct {
		cluster *model.AerospikeCluster
		want    model.TLS
	}{
		"nil cluster":    {cluster: nil, want: model.TLS{}},
		"no TLS block":   {cluster: &model.AerospikeCluster{ClusterLabel: "test-cluster"}, want: model.TLS{}},
		"empty password": {cluster: clusterWithTLS(&withCAFile, nil), want: withCAFile},
		"no credentials": {cluster: &model.AerospikeCluster{TLS: &withCAFile}, want: withCAFile},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := NewClusterTLSResolver(resolver).Resolve(t.Context(), test.cluster)
			require.NoError(t, err)
			require.Equal(t, test.want, result)
		})
	}
}

func TestClusterTLSResolverPropagatesResolverError(t *testing.T) {
	ctrl := gomock.NewController(t)
	resolver := NewMockResolver(ctrl)
	resolver.EXPECT().
		Resolve(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", errors.New("secret agent unreachable"))

	cluster := clusterWithTLS(&model.TLS{KeyfilePassword: "secrets:agent:key"}, nil)

	_, err := NewClusterTLSResolver(resolver).Resolve(t.Context(), cluster)
	require.ErrorContains(t, err, "failed to resolve TLS key-file-password")
}
