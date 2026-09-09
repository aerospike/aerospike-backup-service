package aerospike

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type clusterTLSFiles struct {
	caFile           string
	certFile         string
	encryptedKeyFile string
	keyPassword      string
}

// writeClusterTLSFiles writes a CA cert, a client cert, and a password-encrypted
// client key to a temp dir, mirroring how an Aerospike cluster's mTLS files look
// on disk.
func writeClusterTLSFiles(t *testing.T) clusterTLSFiles {
	t.Helper()
	dir := t.TempDir()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caFile := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600))

	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	clientSerial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	require.NoError(t, err)
	clientTemplate := &x509.Certificate{
		SerialNumber: clientSerial,
		Subject:      pkix.Name{CommonName: "client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	require.NoError(t, err)
	certFile := filepath.Join(dir, "client.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER})
	require.NoError(t, os.WriteFile(certFile, certPEM, 0o600))

	const keyPassword = "correct-horse-battery-staple"
	pemBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)}
	//nolint:staticcheck // DEK-Info PEM encryption is what ABS decrypts for password-protected keys
	encryptedBlock, err := x509.EncryptPEMBlock(
		rand.Reader, pemBlock.Type, pemBlock.Bytes, []byte(keyPassword), x509.PEMCipherAES256)
	require.NoError(t, err)
	encryptedKeyFile := filepath.Join(dir, "client.enc.key")
	require.NoError(t, os.WriteFile(encryptedKeyFile, pem.EncodeToMemory(encryptedBlock), 0o600))

	return clusterTLSFiles{
		caFile:           caFile,
		certFile:         certFile,
		encryptedKeyFile: encryptedKeyFile,
		keyPassword:      keyPassword,
	}
}

// clusterRequiringTLS builds a cluster whose seed node requires TLS (matching
// setTLSConfig's anySeedNodeHasTLSName check) and whose client key password is a
// Secret Agent reference rather than a literal value.
func clusterRequiringTLS(
	files clusterTLSFiles, agent *model.SecretAgent, keyfilePasswordRef string,
) *model.AerospikeCluster {
	return &model.AerospikeCluster{
		ClusterLabel: "test-cluster",
		SeedNodes: []model.SeedNode{
			{HostName: "127.0.0.1", Port: 3000, TLSName: "test-tls-name"},
		},
		Credentials: &model.Credentials{
			AuthMode:    model.AuthModePKI,
			SecretAgent: agent,
		},
		TLS: &model.TLS{
			ClientTLS: model.ClientTLS{
				CAFile:   files.caFile,
				Certfile: files.certFile,
				Keyfile:  files.encryptedKeyFile,
			},
			KeyfilePassword: keyfilePasswordRef,
		},
	}
}

// TestClientPolicyResolvesTLSKeyfilePasswordThroughSecretAgent guards the fix: a
// cluster's TLS.KeyfilePassword can be a Secret Agent reference (see
// dto.TLS.KeyfilePassword), and setTLSConfig must resolve it - the same way
// probe.probeCluster already does - before decrypting the client key.
func TestClientPolicyResolvesTLSKeyfilePasswordThroughSecretAgent(t *testing.T) {
	ctrl := gomock.NewController(t)
	files := writeClusterTLSFiles(t)
	agent := &model.SecretAgent{Address: "127.0.0.1"}
	cluster := clusterRequiringTLS(files, agent, "secrets:agent1:tls-key")

	resolvedTLS := *cluster.TLS
	resolvedTLS.KeyfilePassword = files.keyPassword
	resolver := secrets.NewMockKeyfilePasswordResolver(ctrl)
	resolver.EXPECT().
		Resolve(gomock.Any(), *cluster.TLS, agent).
		Return(resolvedTLS, nil)

	factory := &clientFactory{resolver: resolver}
	policy, err := factory.clientPolicy(t.Context(), cluster)
	require.NoError(t, err)
	require.NotNil(t, policy.TlsConfig, "seed node requires TLS; policy.TlsConfig must not be nil")
}

// TestClientPolicyTLSKeyfilePasswordResolutionErrorPropagates guards the other half
// of the fix: if the Secret Agent can't resolve the password, client creation must
// fail, not silently fall back to a TLS-less policy.TlsConfig.
func TestClientPolicyTLSKeyfilePasswordResolutionErrorPropagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	files := writeClusterTLSFiles(t)
	agent := &model.SecretAgent{Address: "127.0.0.1"}
	cluster := clusterRequiringTLS(files, agent, "secrets:agent1:tls-key")

	resolver := secrets.NewMockKeyfilePasswordResolver(ctrl)
	resolver.EXPECT().
		Resolve(gomock.Any(), *cluster.TLS, agent).
		Return(model.TLS{}, errors.New("secret agent unreachable"))

	factory := &clientFactory{resolver: resolver}
	policy, err := factory.clientPolicy(t.Context(), cluster)
	require.Error(t, err, "a resolution failure must fail client creation, not silently disable TLS")
	assert.Nil(t, policy)
}

// TestClientPolicyTLSConfigErrorPropagates covers the general "don't swallow
// NewTLSConfig errors" half of the fix, independent of the password path.
func TestClientPolicyTLSConfigErrorPropagates(t *testing.T) {
	cluster := &model.AerospikeCluster{
		ClusterLabel: "test-cluster",
		SeedNodes: []model.SeedNode{
			{HostName: "127.0.0.1", Port: 3000, TLSName: "test-tls-name"},
		},
		TLS: &model.TLS{
			ClientTLS: model.ClientTLS{CAFile: "/does/not/exist.pem"},
		},
	}

	// KeyfilePassword is empty, so KeyfilePasswordResolver.Resolve short-circuits
	// without touching its underlying Resolver: nil is a valid resolver here.
	factory := &clientFactory{resolver: secrets.NewKeyfilePasswordResolver(nil)}
	policy, err := factory.clientPolicy(t.Context(), cluster)
	require.Error(t, err)
	assert.Nil(t, policy)
}

// TestClientPolicyNoTLSWhenNoSeedNodeRequiresIt locks in the unrelated early-exit
// path: no seed node advertises a TLS name, so no TLS config - and no Secret Agent
// call - should happen at all.
func TestClientPolicyNoTLSWhenNoSeedNodeRequiresIt(t *testing.T) {
	cluster := &model.AerospikeCluster{
		ClusterLabel: "test-cluster",
		SeedNodes: []model.SeedNode{
			{HostName: "127.0.0.1", Port: 3000},
		},
	}

	factory := &clientFactory{}
	policy, err := factory.clientPolicy(t.Context(), cluster)
	require.NoError(t, err)
	assert.Nil(t, policy.TlsConfig)
}
