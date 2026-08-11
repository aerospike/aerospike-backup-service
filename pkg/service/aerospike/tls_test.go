package aerospike

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	caCertFile     string
	serverCertFile string
	serverKeyFile  string
	encKeyFile     string
	encKeyPassword = "testpassword"
	caCertPEM      []byte
)

func setupCertificates(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()

	// 1. Generate CA Certificate
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2023),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, 1),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "Failed to generate CA private key")

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err, "Failed to create CA certificate")

	caCertFile = filepath.Join(tempDir, "ca.crt")
	caCertPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caBytes})
	require.NoError(t, os.WriteFile(caCertFile, caCertPEM, 0600), "Failed to write CA cert to file")

	// 2. Generate Server/Client Certificate signed by our CA
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2024),
		Subject: pkix.Name{
			Organization: []string{"Test Server"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, 1),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "Failed to generate cert private key")

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err, "Failed to create certificate")

	serverCertFile = filepath.Join(tempDir, "server.crt")
	serverCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	require.NoError(t, os.WriteFile(serverCertFile, serverCertPEM, 0600), "Failed to write server cert file")

	serverKeyFile = filepath.Join(tempDir, "server.key")
	serverKeyPEM := pem.EncodeToMemory(
		&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey)})
	require.NoError(t, os.WriteFile(serverKeyFile, serverKeyPEM, 0600), "Failed to write server key file")

	// 3. Create an encrypted version of the private key
	encKeyFile = filepath.Join(tempDir, "server.enc.key")
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	}
	//nolint:staticcheck // DEK-Info is deprecated but needed for testing password-protected keys
	//noinspection GoDeprecation
	encryptedBlock, err := x509.EncryptPEMBlock(
		rand.Reader, pemBlock.Type, pemBlock.Bytes, []byte(encKeyPassword), x509.PEMCipherAES256)
	require.NoError(t, err, "Failed to encrypt PEM block")
	require.NoError(t, os.WriteFile(encKeyFile, pem.EncodeToMemory(encryptedBlock), 0600))
}

func TestNewTLSConfig(t *testing.T) {
	setupCertificates(t)

	t.Run("Nil TLS Input", func(t *testing.T) {
		cfg, err := NewTLSConfig(nil)
		require.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("Empty TLS Input", func(t *testing.T) {
		cfg, err := NewTLSConfig(&model.TLS{})
		require.NoError(t, err)
		assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion, "Default MinVersion should be TLS 1.2")
	})

	t.Run("With Server Name", func(t *testing.T) {
		serverName := "my.test.server"
		cfg, err := NewTLSConfig(&model.TLS{ClientTLS: model.ClientTLS{Name: &serverName}})
		require.NoError(t, err)
		assert.Equal(t, serverName, cfg.ServerName)
	})

	t.Run("With CAFile and CAPath", func(t *testing.T) {
		caSubDir := filepath.Join(t.TempDir(), "ca_path")
		require.NoError(t, os.Mkdir(caSubDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(caSubDir, "ca.crt"), caCertPEM, 0600))

		cfg, err := NewTLSConfig(&model.TLS{
			ClientTLS: model.ClientTLS{CAFile: &caCertFile},
			CAPath:    &caSubDir,
		})
		require.NoError(t, err)
		//noinspection GoDeprecation
		assert.NotEmpty(t, cfg.RootCAs.Subjects(), "RootCAs should be populated") //nolint:staticcheck
	})

	// This test reproduces the Kubernetes secret volume mount structure
	t.Run("With K8s-style CAPath Symlinks", func(t *testing.T) {
		// 1. Create the base directory (e.g., /etc/aerospike/secret/cacerts/)
		baseDir := t.TempDir()
		caPathK8s := filepath.Join(baseDir, "cacerts")
		require.NoError(t, os.Mkdir(caPathK8s, 0755))

		// 2. Create the actual data directory (e.g., ..2025_10_27_12_34_56_789/)
		actualDataDirName := "..2025_10_27_12_34_56_789"
		actualDataDir := filepath.Join(caPathK8s, actualDataDirName)
		require.NoError(t, os.Mkdir(actualDataDir, 0755))

		// 3. Create the symlink to the data directory (e.g., ..data -> ..2025_10_27_12_34_56_789/)
		symlinkDataDirName := "..data"
		symlinkDataDirPath := filepath.Join(caPathK8s, symlinkDataDirName)
		require.NoError(t, os.Symlink(actualDataDirName, symlinkDataDirPath))

		// 4. Write the certificate files into the *actual* data directory
		cert1Path := filepath.Join(actualDataDir, "cert1.pem")
		require.NoError(t, os.WriteFile(cert1Path, caCertPEM, 0600))

		// 5. Create the file symlinks in the base directory
		// e.g., cert1.pem -> ..data/cert1.pem
		target1 := filepath.Join(symlinkDataDirName, "cert1.pem")
		symlinkCert1Path := filepath.Join(caPathK8s, "cert1.pem")
		require.NoError(t, os.Symlink(target1, symlinkCert1Path))

		// 6. Test NewTLSConfig with this path
		// The loadCertPool function will scan `caPathK8s` and find:
		// - `..2025_10_27_12_34_56_789` (dir, skipped)
		// - `..data` (symlink to dir, skipped)
		// - `cert1.pem` (symlink to file, loaded)
		cfg, err := NewTLSConfig(&model.TLS{CAPath: &caPathK8s})
		require.NoError(t, err)
		require.NotNil(t, cfg.RootCAs, "RootCAs should be populated")

		// 7. Should have one more cert than the system pool
		systemPool, err := x509.SystemCertPool()
		require.NoError(t, err)

		//nolint:staticcheck
		//noinspection GoDeprecation
		assert.Len(t, cfg.RootCAs.Subjects(), len(systemPool.Subjects())+1)
	})

	t.Run("With Client Certs", func(t *testing.T) {
		cfg, err := NewTLSConfig(&model.TLS{
			ClientTLS: model.ClientTLS{
				Certfile: &serverCertFile,
				Keyfile:  &serverKeyFile,
			},
		})
		require.NoError(t, err)
		assert.Len(t, cfg.Certificates, 1)
	})

	t.Run("With Encrypted Client Key", func(t *testing.T) {
		t.Run("Correct Password", func(t *testing.T) {
			cfg, err := NewTLSConfig(&model.TLS{
				ClientTLS: model.ClientTLS{
					Certfile: &serverCertFile,
					Keyfile:  &encKeyFile,
				},
				KeyfilePassword: encKeyPassword,
			})
			require.NoError(t, err)
			assert.Len(t, cfg.Certificates, 1, "Certificate should be loaded with correct password")
		})

		t.Run("Wrong Password", func(t *testing.T) {
			wrongPass := "wrong"
			_, err := NewTLSConfig(&model.TLS{
				ClientTLS: model.ClientTLS{
					Certfile: &serverCertFile,
					Keyfile:  &encKeyFile,
				},
				KeyfilePassword: wrongPass,
			})
			require.Error(t, err, "Should fail with wrong password")
		})

		t.Run("No Password", func(t *testing.T) {
			_, err := NewTLSConfig(&model.TLS{
				ClientTLS: model.ClientTLS{
					Certfile: &serverCertFile,
					Keyfile:  &encKeyFile,
				},
			})
			require.Error(t, err, "Should fail when no password is provided for an encrypted key")
		})
	})
}

// TestParseProtocols provides table-driven tests for protocol parsing logic.
func TestParseProtocols(t *testing.T) {
	testCases := []struct {
		name        string
		protocolStr *string
		wantMin     uint16
		wantMax     uint16
		expectErr   bool
	}{
		{"NilInput", nil, tls.VersionTLS12, 0, false},
		{"EmptyInput", ptr.Of(""), tls.VersionTLS12, 0, false},
		{"WithSpaces", ptr.Of("  TLSv1.2  "), tls.VersionTLS12, tls.VersionTLS12, false},
		{"InvalidVersion", ptr.Of("TLSv1.9"), 0, 0, true},
		{"MixedValidity", ptr.Of("TLSv1.2 BOGUS"), 0, 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			minVersion, maxVersion, err := parseProtocols(tc.protocolStr)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantMin, minVersion, "MinVersion mismatch")
				assert.Equal(t, tc.wantMax, maxVersion, "MaxVersion mismatch")
			}
		})
	}
}

// TestParseCipherSuites provides table-driven tests for cipher suite parsing.
func TestParseCipherSuites(t *testing.T) {
	validSuite1 := "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	validSuite2 := "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
	validSuiteID1 := tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
	validSuiteID2 := tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384

	testCases := []struct {
		name       string
		ciphersStr *string
		wantSuites []uint16
		expectErr  bool
	}{
		{"NilInput", nil, nil, false},
		{"EmptyInput", ptr.Of(""), nil, false},
		{"SingleSuite", ptr.Of(validSuite1), []uint16{validSuiteID1}, false},
		{"MultipleSuites",
			ptr.Of(fmt.Sprintf("%s:%s", validSuite1, validSuite2)),
			[]uint16{validSuiteID1, validSuiteID2}, false},
		{"WithWhitespace",
			ptr.Of(fmt.Sprintf(" %s : %s ", validSuite1, validSuite2)),
			[]uint16{validSuiteID1, validSuiteID2}, false},
		{"InvalidSuiteName", ptr.Of("TLS_BOGUS_SUITE"), nil, true},
		{"MixedValidity",
			ptr.Of(validSuite1 + ":TLS_BOGUS_SUITE"), nil, true},
		{"EmptyStringBetweenColons",
			ptr.Of(fmt.Sprintf("%s::%s", validSuite1, validSuite2)),
			[]uint16{validSuiteID1, validSuiteID2}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			suites, err := parseCipherSuites(tc.ciphersStr)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantSuites, suites)
			}
		})
	}
}
