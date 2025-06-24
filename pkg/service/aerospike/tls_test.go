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
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
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
	encryptedBlock, err := x509.EncryptPEMBlock(
		rand.Reader, pemBlock.Type, pemBlock.Bytes, []byte(encKeyPassword), x509.PEMCipherAES256)
	require.NoError(t, err, "Failed to encrypt PEM block")
	require.NoError(t, os.WriteFile(encKeyFile, pem.EncodeToMemory(encryptedBlock), 0600))
}

func TestNewTLSConfig(t *testing.T) {
	setupCertificates(t)

	t.Run("Nil TLS Input", func(t *testing.T) {
		cfg, err := NewTLSConfig(nil)
		assert.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("Empty TLS Input", func(t *testing.T) {
		cfg, err := NewTLSConfig(&model.TLS{})
		require.NoError(t, err)
		assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion, "Default MinVersion should be TLS 1.2")
	})

	t.Run("With Server Name", func(t *testing.T) {
		serverName := "my.test.server"
		cfg, err := NewTLSConfig(&model.TLS{Name: &serverName})
		require.NoError(t, err)
		assert.Equal(t, serverName, cfg.ServerName)
	})

	t.Run("With CAFile and CAPath", func(t *testing.T) {
		caSubDir := filepath.Join(t.TempDir(), "ca_path")
		require.NoError(t, os.Mkdir(caSubDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(caSubDir, "ca.crt"), caCertPEM, 0600))

		cfg, err := NewTLSConfig(&model.TLS{CAFile: &caCertFile, CAPath: &caSubDir})
		require.NoError(t, err)
		assert.NotEmpty(t, cfg.RootCAs.Subjects(), "RootCAs should be populated") //nolint:staticcheck
	})

	t.Run("With Client Certs", func(t *testing.T) {
		cfg, err := NewTLSConfig(&model.TLS{
			Certfile: &serverCertFile,
			Keyfile:  &serverKeyFile,
		})
		require.NoError(t, err)
		assert.Len(t, cfg.Certificates, 1)
	})

	t.Run("With Encrypted Client Key", func(t *testing.T) {
		t.Run("Correct Password", func(t *testing.T) {
			cfg, err := NewTLSConfig(&model.TLS{
				Certfile:        &serverCertFile,
				Keyfile:         &encKeyFile,
				KeyfilePassword: &encKeyPassword,
			})
			require.NoError(t, err)
			assert.Len(t, cfg.Certificates, 1, "Certificate should be loaded with correct password")
		})

		t.Run("Wrong Password", func(t *testing.T) {
			wrongPass := "wrong"
			_, err := NewTLSConfig(&model.TLS{
				Certfile:        &serverCertFile,
				Keyfile:         &encKeyFile,
				KeyfilePassword: &wrongPass,
			})
			assert.Error(t, err, "Should fail with wrong password")
		})

		t.Run("No Password", func(t *testing.T) {
			_, err := NewTLSConfig(&model.TLS{
				Certfile: &serverCertFile,
				Keyfile:  &encKeyFile,
			})
			assert.Error(t, err, "Should fail when no password is provided for an encrypted key")
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
		{"EmptyInput", util.Ptr(""), tls.VersionTLS12, 0, false},
		{"WithSpaces", util.Ptr("  TLSv1.2  "), tls.VersionTLS12, tls.VersionTLS12, false},
		{"InvalidVersion", util.Ptr("TLSv1.9"), 0, 0, true},
		{"MixedValidity", util.Ptr("TLSv1.2 BOGUS"), 0, 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			minVersion, maxVersion, err := parseProtocols(tc.protocolStr)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
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
		{"EmptyInput", util.Ptr(""), nil, false},
		{"SingleSuite", util.Ptr(validSuite1), []uint16{validSuiteID1}, false},
		{"MultipleSuites",
			util.Ptr(fmt.Sprintf("%s:%s", validSuite1, validSuite2)),
			[]uint16{validSuiteID1, validSuiteID2}, false},
		{"WithWhitespace",
			util.Ptr(fmt.Sprintf(" %s : %s ", validSuite1, validSuite2)),
			[]uint16{validSuiteID1, validSuiteID2}, false},
		{"InvalidSuiteName", util.Ptr("TLS_BOGUS_SUITE"), nil, true},
		{"MixedValidity",
			util.Ptr(fmt.Sprintf("%s:TLS_BOGUS_SUITE", validSuite1)), nil, true},
		{"EmptyStringBetweenColons",
			util.Ptr(fmt.Sprintf("%s::%s", validSuite1, validSuite2)),
			[]uint16{validSuiteID1, validSuiteID2}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			suites, err := parseCipherSuites(tc.ciphersStr)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantSuites, suites)
			}
		})
	}
}
