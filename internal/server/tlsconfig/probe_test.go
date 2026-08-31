package tlsconfig

import (
	"path/filepath"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestProbeHTTPS(t *testing.T) {
	files := createTestCertificateFiles(t)

	t.Run("valid", func(t *testing.T) {
		err := ProbeHTTPS(t.Context(), &model.ServerConfigHTTPS{
			CertFile: files.certFile,
			KeyFile:  files.keyFile,
		}, newTestResolver(t))
		require.NoError(t, err)
	})

	t.Run("missing certificate", func(t *testing.T) {
		err := ProbeHTTPS(t.Context(), &model.ServerConfigHTTPS{
			CertFile: filepath.Join(t.TempDir(), "missing.pem"),
			KeyFile:  files.keyFile,
		}, newTestResolver(t))
		require.ErrorContains(t, err, "failed to load HTTPS certificate and key")
	})

	t.Run("mismatched key pair", func(t *testing.T) {
		other := createTestCertificateFiles(t)
		err := ProbeHTTPS(t.Context(), &model.ServerConfigHTTPS{
			CertFile: files.certFile,
			KeyFile:  other.keyFile,
		}, newTestResolver(t))
		require.ErrorContains(t, err, "private key does not match public key")
	})

	t.Run("disabled skips missing files", func(t *testing.T) {
		err := ProbeHTTPS(t.Context(), &model.ServerConfigHTTPS{
			ListenerConfig: model.ListenerConfig{Disabled: true},
			CertFile:       filepath.Join(t.TempDir(), "missing.pem"),
			KeyFile:        filepath.Join(t.TempDir(), "missing-key.pem"),
		}, newTestResolver(t))
		require.NoError(t, err)
	})
}

func TestProbeCluster(t *testing.T) {
	files := createTestCertificateFiles(t)

	t.Run("valid", func(t *testing.T) {
		err := ProbeCluster(t.Context(), &model.AerospikeCluster{
			TLS: &model.TLS{ClientTLS: model.ClientTLS{
				CAFile:   files.caFile,
				Certfile: files.certFile,
				Keyfile:  files.keyFile,
			}},
		}, nil, newTestResolver(t))
		require.NoError(t, err)
	})

	t.Run("missing CA", func(t *testing.T) {
		err := ProbeCluster(t.Context(), &model.AerospikeCluster{
			TLS: &model.TLS{ClientTLS: model.ClientTLS{
				CAFile: filepath.Join(t.TempDir(), "missing.pem"),
			}},
		}, nil, newTestResolver(t))
		require.Error(t, err)
	})

	t.Run("mismatched key pair", func(t *testing.T) {
		other := createTestCertificateFiles(t)
		err := ProbeCluster(t.Context(), &model.AerospikeCluster{
			TLS: &model.TLS{ClientTLS: model.ClientTLS{
				Certfile: files.certFile,
				Keyfile:  other.keyFile,
			}},
		}, nil, newTestResolver(t))
		require.ErrorContains(t, err, "private key does not match public key")
	})
}

func TestProbeConfigReportsClusterName(t *testing.T) {
	config := model.NewConfig()
	require.NoError(t, config.AddCluster("broken", &model.AerospikeCluster{
		TLS: &model.TLS{ClientTLS: model.ClientTLS{
			CAFile: filepath.Join(t.TempDir(), "missing.pem"),
		}},
	}))

	err := ProbeConfig(t.Context(), config, newTestResolver(t))
	require.ErrorContains(t, err, `cluster "broken" TLS validation failed`)
}
