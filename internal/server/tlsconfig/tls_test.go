package tlsconfig

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
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
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	require.NoError(t, err)
	serverTemplate := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
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

func newTestResolver(t *testing.T) secrets.Resolver {
	t.Helper()

	resolver := secrets.NewMockResolver(gomock.NewController(t))
	resolver.EXPECT().
		Resolve(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ *model.SecretAgent, value string) (string, error) {
			return value, nil
		}).
		AnyTimes()

	return resolver
}

// loadTLSConfig performs the caller-side sequence: load the key pair, then build the config.
func loadTLSConfig(t *testing.T, cfg *model.ServerConfigHTTPS, resolver secrets.Resolver) (*tls.Config, error) {
	t.Helper()

	reloader := NewCertificateReloader(cfg, resolver, DefaultWatchInterval)
	if err := reloader.Load(t.Context()); err != nil {
		return nil, err
	}

	return NewTLSConfig(cfg, reloader.GetCertificate)
}

func requireTLSConfig(t *testing.T, cfg *model.ServerConfigHTTPS, resolver secrets.Resolver) *tls.Config {
	t.Helper()

	tlsCfg, err := loadTLSConfig(t, cfg, resolver)
	require.NoError(t, err)

	return tlsCfg
}

func servedCertificate(t *testing.T, config *tls.Config) *tls.Certificate {
	t.Helper()

	cert, err := config.GetCertificate(&tls.ClientHelloInfo{})
	require.NoError(t, err)
	require.NotNil(t, cert)

	return cert
}

func TestNew(t *testing.T) {
	files := createTestCertificateFiles(t)

	t.Run("secure defaults", func(t *testing.T) {
		config := requireTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile: files.certFile,
			KeyFile:  files.keyFile,
		}, newTestResolver(t))

		assert.Equal(t, uint16(tls.VersionTLS12), config.MinVersion)
		assert.Nil(t, config.CipherSuites)
		assert.Equal(t, tls.NoClientCert, config.ClientAuth)
		assert.NotNil(t, servedCertificate(t, config))
		assert.Nil(t, config.Certificates)
	})

	t.Run("explicit secure settings and client CA", func(t *testing.T) {
		config := requireTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:     files.certFile,
			KeyFile:      files.keyFile,
			MinVersion:   "1.3",
			CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
			ClientCAFile: files.caFile,
			ClientAuth:   model.TLSClientAuthRequireAndVerify,
		}, newTestResolver(t))

		assert.Equal(t, uint16(tls.VersionTLS13), config.MinVersion)
		assert.Equal(t, []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}, config.CipherSuites)
		assert.Equal(t, tls.RequireAndVerifyClientCert, config.ClientAuth)
		assert.NotNil(t, config.ClientCAs)
	})

	t.Run("encrypted private key", func(t *testing.T) {
		encryptedKey := writeEncryptedKey(t, files.keyFile, "password")

		config := requireTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:        files.certFile,
			KeyFile:         encryptedKey,
			KeyFilePassword: "password",
		}, newTestResolver(t))
		assert.NotNil(t, servedCertificate(t, config))
	})

	t.Run("mismatched certificate and key", func(t *testing.T) {
		other := createTestCertificateFiles(t)
		_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile: files.certFile,
			KeyFile:  other.keyFile,
		}, newTestResolver(t))
		require.ErrorContains(t, err, "private key does not match public key")
	})

	t.Run("client authentication without CA", func(t *testing.T) {
		_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:   files.certFile,
			KeyFile:    files.keyFile,
			ClientAuth: model.TLSClientAuthRequireAndVerify,
		}, newTestResolver(t))
		require.ErrorContains(t, err, "requires a client CA file")
	})

	t.Run("missing certificate", func(t *testing.T) {
		_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile: filepath.Join(t.TempDir(), "missing.pem"),
			KeyFile:  files.keyFile,
		}, newTestResolver(t))
		require.ErrorContains(t, err, "failed to load HTTPS certificate and key")
	})

	t.Run("unsupported minimum version", func(t *testing.T) {
		_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:   files.certFile,
			KeyFile:    files.keyFile,
			MinVersion: "1.1",
		}, newTestResolver(t))
		require.ErrorContains(t, err, "unsupported minimum TLS version")
	})

	t.Run("unsupported cipher suite", func(t *testing.T) {
		_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:     files.certFile,
			KeyFile:      files.keyFile,
			CipherSuites: []string{"NOT_A_SUITE"},
		}, newTestResolver(t))
		require.ErrorContains(t, err, "unsupported or insecure TLS cipher suite")
	})

	t.Run("unsupported client authentication", func(t *testing.T) {
		_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:   files.certFile,
			KeyFile:    files.keyFile,
			ClientAuth: "verify-if-given",
		}, newTestResolver(t))
		require.ErrorContains(t, err, "unsupported TLS client authentication mode")
	})

	t.Run("password-protected unencrypted key", func(t *testing.T) {
		config := requireTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:        files.certFile,
			KeyFile:         files.keyFile,
			KeyFilePassword: "unused",
		}, newTestResolver(t))
		assert.NotNil(t, servedCertificate(t, config))
	})

	t.Run("wrong key password", func(t *testing.T) {
		encryptedKey := writeEncryptedKey(t, files.keyFile, "password")
		_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:        files.certFile,
			KeyFile:         encryptedKey,
			KeyFilePassword: "wrong",
		}, newTestResolver(t))
		require.ErrorContains(t, err, "failed to decrypt HTTPS private key")
	})

	t.Run("missing certificate with password", func(t *testing.T) {
		_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:        filepath.Join(t.TempDir(), "missing.pem"),
			KeyFile:         files.keyFile,
			KeyFilePassword: "password",
		}, newTestResolver(t))
		require.ErrorContains(t, err, "failed to read HTTPS certificate")
	})

	t.Run("missing key with password", func(t *testing.T) {
		_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:        files.certFile,
			KeyFile:         filepath.Join(t.TempDir(), "missing-key.pem"),
			KeyFilePassword: "password",
		}, newTestResolver(t))
		require.ErrorContains(t, err, "failed to read HTTPS private key")
	})

	t.Run("non-PEM key with password", func(t *testing.T) {
		keyFile := filepath.Join(t.TempDir(), "not-pem.key")
		require.NoError(t, os.WriteFile(keyFile, []byte("not a pem block"), 0o600))
		_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:        files.certFile,
			KeyFile:         keyFile,
			KeyFilePassword: "password",
		}, newTestResolver(t))
		require.ErrorContains(t, err, "failed to decode PEM block")
	})

	t.Run("password with mismatched key pair", func(t *testing.T) {
		other := createTestCertificateFiles(t)
		_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:        files.certFile,
			KeyFile:         other.keyFile,
			KeyFilePassword: "unused",
		}, newTestResolver(t))
		require.ErrorContains(t, err, "failed to load HTTPS certificate and key")
	})

	t.Run("missing client CA file", func(t *testing.T) {
		_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:     files.certFile,
			KeyFile:      files.keyFile,
			ClientCAFile: filepath.Join(t.TempDir(), "missing-ca.pem"),
			ClientAuth:   model.TLSClientAuthRequest,
		}, newTestResolver(t))
		require.ErrorContains(t, err, "failed to read client CA file")
	})

	t.Run("client CA file with no certificates", func(t *testing.T) {
		caFile := filepath.Join(t.TempDir(), "empty-ca.pem")
		require.NoError(t, os.WriteFile(caFile, []byte("not a certificate"), 0o600))
		_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
			CertFile:     files.certFile,
			KeyFile:      files.keyFile,
			ClientCAFile: caFile,
			ClientAuth:   model.TLSClientAuthRequest,
		}, newTestResolver(t))
		require.ErrorContains(t, err, "contains no certificates")
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

	config := requireTLSConfig(t, &model.ServerConfigHTTPS{
		CertFile:        files.certFile,
		KeyFile:         encryptedKey,
		KeyFilePassword: "secrets:agent1:tls-key",
		SecretAgent:     agent,
	}, resolver)
	assert.NotNil(t, servedCertificate(t, config))
}

func TestNewReturnsSecretAgentResolutionError(t *testing.T) {
	files := createTestCertificateFiles(t)

	ctrl := gomock.NewController(t)
	resolver := secrets.NewMockResolver(ctrl)
	resolver.EXPECT().
		Resolve(gomock.Any(), gomock.Any(), "secrets:agent1:tls-key").
		Return("", assert.AnError)

	_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
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
