package tlsconfig

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// newPassthroughTLSResolver hands a cluster's TLS config back unchanged. Resolution
// itself is covered by secrets.ClusterTLSResolver's own tests; these tests are about
// what prober does with the result.
func newPassthroughTLSResolver(t *testing.T) secrets.ClusterTLSResolver {
	t.Helper()

	resolver := secrets.NewMockClusterTLSResolver(gomock.NewController(t))
	resolver.EXPECT().
		Resolve(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cluster *model.AerospikeCluster) (model.TLS, error) {
			if cluster == nil || cluster.TLS == nil {
				return model.TLS{}, nil
			}

			return *cluster.TLS, nil
		}).
		AnyTimes()

	return resolver
}

func TestProbeCluster(t *testing.T) {
	files := createTestCertificateFiles(t)
	prober := &prober{resolver: newPassthroughTLSResolver(t)}

	t.Run("valid", func(t *testing.T) {
		err := prober.probeCluster(t.Context(), &model.AerospikeCluster{
			TLS: &model.TLS{ClientTLS: model.ClientTLS{
				CAFile:   files.caFile,
				Certfile: files.certFile,
				Keyfile:  files.keyFile,
			}},
		})
		require.NoError(t, err)
	})

	t.Run("missing CA", func(t *testing.T) {
		err := prober.probeCluster(t.Context(), &model.AerospikeCluster{
			TLS: &model.TLS{ClientTLS: model.ClientTLS{
				CAFile: filepath.Join(t.TempDir(), "missing.pem"),
			}},
		})
		require.Error(t, err)
	})

	t.Run("mismatched key pair", func(t *testing.T) {
		other := createTestCertificateFiles(t)
		err := prober.probeCluster(t.Context(), &model.AerospikeCluster{
			TLS: &model.TLS{ClientTLS: model.ClientTLS{
				Certfile: files.certFile,
				Keyfile:  other.keyFile,
			}},
		})
		require.ErrorContains(t, err, "private key does not match public key")
	})
}

func TestProbeReportsClusterName(t *testing.T) {
	config := model.NewConfig()
	require.NoError(t, config.AddCluster("broken", &model.AerospikeCluster{
		TLS: &model.TLS{ClientTLS: model.ClientTLS{
			CAFile: filepath.Join(t.TempDir(), "missing.pem"),
		}},
	}))

	err := NewProber(newPassthroughTLSResolver(t)).Probe(t.Context(), config)
	require.ErrorContains(t, err, `cluster "broken" TLS validation failed`)
}

func TestProbeSecretAgent(t *testing.T) {
	files := createTestCertificateFiles(t)
	prober := NewProber(newPassthroughTLSResolver(t))

	t.Run("valid", func(t *testing.T) {
		config := model.NewConfig()
		require.NoError(t, config.AddSecretAgent("agent", &model.SecretAgent{
			ClientTLS: model.ClientTLS{
				CAFile:   files.caFile,
				Name:     "secret-agent",
				Certfile: files.certFile,
				Keyfile:  files.keyFile,
			},
		}))
		require.NoError(t, prober.Probe(t.Context(), config))
	})

	t.Run("missing CA", func(t *testing.T) {
		config := model.NewConfig()
		require.NoError(t, config.AddSecretAgent("broken", &model.SecretAgent{
			ClientTLS: model.ClientTLS{
				CAFile: filepath.Join(t.TempDir(), "missing.pem"),
			},
		}))
		err := prober.Probe(t.Context(), config)
		require.ErrorContains(t, err, `secret agent "broken" TLS validation failed`)
	})

	t.Run("no TLS files", func(t *testing.T) {
		config := model.NewConfig()
		require.NoError(t, config.AddSecretAgent("plain", &model.SecretAgent{
			Address: "localhost",
		}))
		require.NoError(t, prober.Probe(t.Context(), config))
	})

	t.Run("inline agent of cluster", func(t *testing.T) {
		config := model.NewConfig()
		require.NoError(t, config.AddCluster("source", &model.AerospikeCluster{
			Credentials: &model.Credentials{
				SecretAgent: &model.SecretAgent{
					ClientTLS: model.ClientTLS{
						CAFile: filepath.Join(t.TempDir(), "missing.pem"),
					},
				},
			},
		}))

		err := prober.Probe(t.Context(), config)
		require.ErrorContains(t, err, `secret agent of cluster "source" TLS validation failed`)
	})

	t.Run("inline agent of HTTPS listener", func(t *testing.T) {
		config := model.NewConfig()
		config.ServiceConfig.ServerHTTPS = &model.ServerConfigHTTPS{
			SecretAgent: &model.SecretAgent{
				ClientTLS: model.ClientTLS{
					CAFile: filepath.Join(t.TempDir(), "missing.pem"),
				},
			},
		}

		err := prober.Probe(t.Context(), config)
		require.ErrorContains(t, err, "secret agent of the HTTPS listener TLS validation failed")
	})
}
