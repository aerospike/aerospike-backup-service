package tlsconfig

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
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

type testCertificateFiles struct {
	certFile string
	keyFile  string
	caFile   string
}

func createTestCertificateFiles(t *testing.T) testCertificateFiles {
	t.Helper()

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

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(
		rand.Reader, serverTemplate, caTemplate, &serverKey.PublicKey, caKey,
	)
	require.NoError(t, err)

	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	certFile := filepath.Join(dir, "server.pem")
	keyFile := filepath.Join(dir, "server-key.pem")
	require.NoError(t, os.WriteFile(
		caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600,
	))
	require.NoError(t, os.WriteFile(
		certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER}), 0600,
	))
	require.NoError(t, os.WriteFile(
		keyFile,
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)}),
		0600,
	))

	return testCertificateFiles{certFile: certFile, keyFile: keyFile, caFile: caFile}
}

func TestNew(t *testing.T) {
	files := createTestCertificateFiles(t)

	t.Run("secure defaults", func(t *testing.T) {
		config, err := NewTLSConfig(t.Context(), &model.ServerConfigHTTPS{
			CertFile: files.certFile,
			KeyFile:  files.keyFile,
		}, nil)
		require.NoError(t, err)

		assert.Equal(t, uint16(tls.VersionTLS12), config.MinVersion)
		assert.Nil(t, config.CipherSuites)
		assert.Equal(t, tls.NoClientCert, config.ClientAuth)
		assert.Len(t, config.Certificates, 1)
	})

	t.Run("explicit secure settings and client CA", func(t *testing.T) {
		config, err := NewTLSConfig(t.Context(), &model.ServerConfigHTTPS{
			CertFile:     files.certFile,
			KeyFile:      files.keyFile,
			MinVersion:   "1.3",
			CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
			ClientCAFile: files.caFile,
			ClientAuth:   model.TLSClientAuthRequireAndVerify,
		}, nil)
		require.NoError(t, err)

		assert.Equal(t, uint16(tls.VersionTLS13), config.MinVersion)
		assert.Equal(t, []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}, config.CipherSuites)
		assert.Equal(t, tls.RequireAndVerifyClientCert, config.ClientAuth)
		assert.NotNil(t, config.ClientCAs)
	})

	t.Run("encrypted private key", func(t *testing.T) {
		encryptedKey := writeEncryptedKey(t, files.keyFile, "password")

		config, err := NewTLSConfig(t.Context(), &model.ServerConfigHTTPS{
			CertFile:        files.certFile,
			KeyFile:         encryptedKey,
			KeyFilePassword: "password",
		}, nil)
		require.NoError(t, err)
		assert.Len(t, config.Certificates, 1)
	})

	t.Run("mismatched certificate and key", func(t *testing.T) {
		other := createTestCertificateFiles(t)
		_, err := NewTLSConfig(t.Context(), &model.ServerConfigHTTPS{
			CertFile: files.certFile,
			KeyFile:  other.keyFile,
		}, nil)
		require.ErrorContains(t, err, "private key does not match public key")
	})

	t.Run("client authentication without CA", func(t *testing.T) {
		_, err := NewTLSConfig(t.Context(), &model.ServerConfigHTTPS{
			CertFile:   files.certFile,
			KeyFile:    files.keyFile,
			ClientAuth: model.TLSClientAuthRequireAndVerify,
		}, nil)
		require.ErrorContains(t, err, "requires a client CA file")
	})
}

func TestNewResolvesKeyFilePasswordThroughSecretAgent(t *testing.T) {
	files := createTestCertificateFiles(t)
	encryptedKey := writeEncryptedKey(t, files.keyFile, "resolved-password")
	agent := &model.SecretAgent{Address: "127.0.0.1"}

	ctrl := gomock.NewController(t)
	resolver := secrets.NewMockResolver(ctrl)
	resolver.EXPECT().
		Resolve(gomock.Any(), agent, "secrets:agent1:tls-key").
		Return("resolved-password", nil)

	config, err := NewTLSConfig(t.Context(), &model.ServerConfigHTTPS{
		CertFile:        files.certFile,
		KeyFile:         encryptedKey,
		KeyFilePassword: "secrets:agent1:tls-key",
		SecretAgent:     agent,
	}, resolver)
	require.NoError(t, err)
	assert.Len(t, config.Certificates, 1)
}

func TestNewReturnsSecretAgentResolutionError(t *testing.T) {
	files := createTestCertificateFiles(t)

	ctrl := gomock.NewController(t)
	resolver := secrets.NewMockResolver(ctrl)
	resolver.EXPECT().
		Resolve(gomock.Any(), gomock.Any(), "secrets:agent1:tls-key").
		Return("", assert.AnError)

	_, err := NewTLSConfig(t.Context(), &model.ServerConfigHTTPS{
		CertFile:        files.certFile,
		KeyFile:         files.keyFile,
		KeyFilePassword: "secrets:agent1:tls-key",
		SecretAgent:     &model.SecretAgent{Address: "127.0.0.1"},
	}, resolver)
	require.ErrorContains(t, err, "failed to resolve HTTPS key-file-password")
}

func writeEncryptedKey(t *testing.T, keyFile, password string) string {
	t.Helper()

	keyPEM, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	keyBlock, _ := pem.Decode(keyPEM)
	require.NotNil(t, keyBlock)
	//nolint:staticcheck // Verify compatibility with legacy password-protected PEM keys.
	encryptedBlock, err := x509.EncryptPEMBlock(
		rand.Reader, keyBlock.Type, keyBlock.Bytes, []byte(password), x509.PEMCipherAES256,
	)
	require.NoError(t, err)
	encryptedKey, err := os.CreateTemp(t.TempDir(), "encrypted-key-*.pem")
	require.NoError(t, err)
	_, err = encryptedKey.Write(pem.EncodeToMemory(encryptedBlock))
	require.NoError(t, err)
	require.NoError(t, encryptedKey.Close())

	return encryptedKey.Name()
}

func TestParseCipherSuitesExcludesEveryInsecureSuite(t *testing.T) {
	for _, suite := range tls.InsecureCipherSuites() {
		t.Run(suite.Name, func(t *testing.T) {
			_, err := parseCipherSuites([]string{suite.Name})
			require.Error(t, err)
		})
	}
}

func TestSecureCipherSuiteMapDoesNotContainInsecureSuites(t *testing.T) {
	for _, suite := range tls.InsecureCipherSuites() {
		_, exists := secureCipherSuites[suite.Name]
		assert.Falsef(t, exists, "insecure cipher suite %s must not be accepted", suite.Name)
	}
}
