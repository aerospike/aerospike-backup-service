package dto

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

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
)

// testCertificates holds paths to test certificate files.
type testCertificates struct {
	caFile   string
	caDir    string
	certFile string
	keyFile  string
	tempDir  string
}

// setupTestCertificates creates temporary certificate files for testing.
func setupTestCertificates(t *testing.T) *testCertificates {
	t.Helper()

	tempDir := t.TempDir()

	// Generate a test CA certificate
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate CA key: %v", err)
	}

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
			Country:      []string{"US"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("Failed to create CA certificate: %v", err)
	}

	// Write CA certificate file
	caFile := filepath.Join(tempDir, "ca.pem")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	if err := os.WriteFile(caFile, caPEM, 0600); err != nil {
		t.Fatalf("Failed to write CA file: %v", err)
	}

	// Create CA directory and copy CA file there
	caDir := filepath.Join(tempDir, "ca")
	if err := os.MkdirAll(caDir, 0755); err != nil {
		t.Fatalf("Failed to create CA directory: %v", err)
	}
	caDirFile := filepath.Join(caDir, "ca.pem")
	if err := os.WriteFile(caDirFile, caPEM, 0600); err != nil {
		t.Fatalf("Failed to write CA file to directory: %v", err)
	}

	// Generate client certificate and key
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate client key: %v", err)
	}

	clientTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"Test Client"},
			Country:      []string{"US"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, &clientTemplate, &caTemplate, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("Failed to create client certificate: %v", err)
	}

	// Write client certificate file
	certFile := filepath.Join(tempDir, "client.pem")
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	if err := os.WriteFile(certFile, clientCertPEM, 0600); err != nil {
		t.Fatalf("Failed to write client certificate file: %v", err)
	}

	// Write client key file
	keyFile := filepath.Join(tempDir, "client-key.pem")
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)})
	if err := os.WriteFile(keyFile, clientKeyPEM, 0600); err != nil {
		t.Fatalf("Failed to write client key file: %v", err)
	}

	return &testCertificates{
		caFile:   caFile,
		caDir:    caDir,
		certFile: certFile,
		keyFile:  keyFile,
		tempDir:  tempDir,
	}
}

// cleanupTestCertificates removes temporary certificate files.
func cleanupTestCertificates(certs *testCertificates) {
	if certs != nil && certs.tempDir != "" {
		_ = os.RemoveAll(certs.tempDir)
	}
}

func TestTLS_Validate(t *testing.T) {
	certs := setupTestCertificates(t)
	defer cleanupTestCertificates(certs)

	tests := []struct {
		name    string
		tls     *TLS
		wantErr bool
		errType error
	}{
		{
			name:    "nil TLS should be valid",
			tls:     nil,
			wantErr: false,
		},
		{
			name:    "empty TLS should be valid",
			tls:     &TLS{},
			wantErr: false,
		},
		{
			name: "valid TLS with CAFile only",
			tls: &TLS{
				CAFile: &certs.caFile,
			},
			wantErr: false,
		},
		{
			name: "valid TLS with CAPath only",
			tls: &TLS{
				CAPath: &certs.caDir,
			},
			wantErr: false,
		},
		{
			name: "CAFile and CAPath are mutually exclusive",
			tls: &TLS{
				CAFile: &certs.caFile,
				CAPath: &certs.caDir,
			},
			wantErr: true,
			errType: errMutuallyExclusive,
		},
		{
			name: "valid complete mTLS configuration",
			tls: &TLS{
				Name:     ptr.Of("tls-name"),
				Keyfile:  &certs.keyFile,
				Certfile: &certs.certFile,
			},
			wantErr: false,
		},
		{
			name: "valid mTLS with CA and password",
			tls: &TLS{
				CAFile:          &certs.caFile,
				Name:            ptr.Of("tls-name"),
				Keyfile:         &certs.keyFile,
				KeyfilePassword: ptr.Of(""), // Empty password for unencrypted key
				Certfile:        &certs.certFile,
			},
			wantErr: false,
		},
		{
			name: "mTLS missing name",
			tls: &TLS{
				Keyfile:  &certs.keyFile,
				Certfile: &certs.certFile,
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "mTLS missing keyfile",
			tls: &TLS{
				Name:     ptr.Of("tls-name"),
				Certfile: &certs.certFile,
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "mTLS missing certfile",
			tls: &TLS{
				Name:    ptr.Of("tls-name"),
				Keyfile: &certs.keyFile,
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "keyfile password without keyfile",
			tls: &TLS{
				KeyfilePassword: ptr.Of("password"),
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "only name set should fail",
			tls: &TLS{
				Name: ptr.Of("tls-name"),
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "only keyfile set should fail",
			tls: &TLS{
				Keyfile: &certs.keyFile,
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "only certfile set should fail",
			tls: &TLS{
				Certfile: &certs.certFile,
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "valid TLS with protocols and cipher suite",
			tls: &TLS{
				Protocols:   ptr.Of("TLSv1.2"),
				CipherSuite: ptr.Of("TLS_AES_128_GCM_SHA256"),
			},
			wantErr: false,
		},
		{
			name: "complex valid configuration",
			tls: &TLS{
				CAFile:      &certs.caFile,
				Protocols:   ptr.Of("TLSv1.2"),
				CipherSuite: ptr.Of("TLS_AES_128_GCM_SHA256"),
				Name:        ptr.Of("tls-name"),
				Keyfile:     &certs.keyFile,
				Certfile:    &certs.certFile,
			},
			wantErr: false,
		},
		{
			name: "invalid CA file path",
			tls: &TLS{
				CAFile: ptr.Of("/nonexistent/path/ca.pem"),
			},
			wantErr: true,
		},
		{
			name: "invalid key file path",
			tls: &TLS{
				Name:     ptr.Of("tls-name"),
				Keyfile:  ptr.Of("/nonexistent/path/key.pem"),
				Certfile: &certs.certFile,
			},
			wantErr: true,
		},
		{
			name: "invalid cert file path",
			tls: &TLS{
				Name:     ptr.Of("tls-name"),
				Keyfile:  &certs.keyFile,
				Certfile: ptr.Of("/nonexistent/path/cert.pem"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tls.Validate()

			if tt.wantErr {
				assert.Error(t, err)

				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("expected error type %v, got %v", tt.errType, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTLS_validateCACertificates(t *testing.T) {
	certs := setupTestCertificates(t)
	defer cleanupTestCertificates(certs)

	tests := []struct {
		name    string
		tls     *TLS
		wantErr bool
	}{
		{
			name:    "no CA settings",
			tls:     &TLS{},
			wantErr: false,
		},
		{
			name: "only CAFile",
			tls: &TLS{
				CAFile: &certs.caFile,
			},
			wantErr: false,
		},
		{
			name: "only CAPath",
			tls: &TLS{
				CAPath: &certs.caDir,
			},
			wantErr: false,
		},
		{
			name: "both CAFile and CAPath",
			tls: &TLS{
				CAFile: &certs.caFile,
				CAPath: &certs.caDir,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tls.validateCACertificates()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
