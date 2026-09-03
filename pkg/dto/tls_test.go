package dto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "Failed to generate CA key")

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
	require.NoError(t, err, "Failed to create CA certificate")

	// Write CA certificate file
	caFile := filepath.Join(tempDir, "ca.pem")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	require.NoError(t, os.WriteFile(caFile, caPEM, 0600), "Failed to write CA file")

	// Create CA directory and copy CA file there
	caDir := filepath.Join(tempDir, "ca")
	require.NoError(t, os.MkdirAll(caDir, 0755), "Failed to create CA directory")
	caDirFile := filepath.Join(caDir, "ca.pem")
	require.NoError(t, os.WriteFile(caDirFile, caPEM, 0600), "Failed to write CA file to directory")

	// Generate client certificate and key
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "Failed to generate client key")

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
	require.NoError(t, err, "Failed to create client certificate")

	// Write client certificate file
	certFile := filepath.Join(tempDir, "client.pem")
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	require.NoError(t, os.WriteFile(certFile, clientCertPEM, 0600), "Failed to write client certificate file")

	// Write client key file
	keyFile := filepath.Join(tempDir, "client-key.pem")
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)})
	require.NoError(t, os.WriteFile(keyFile, clientKeyPEM, 0600), "Failed to write client key file")

	return &testCertificates{
		caFile:   caFile,
		caDir:    caDir,
		certFile: certFile,
		keyFile:  keyFile,
		tempDir:  tempDir,
	}
}

func TestTLS_Validate(t *testing.T) {
	certs := setupTestCertificates(t)

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
				ClientTLS: ClientTLS{
					CAFile: certs.caFile,
				},
			},
			wantErr: false,
		},
		{
			name: "valid TLS with CAPath only",
			tls: &TLS{
				CAPath: certs.caDir,
			},
			wantErr: false,
		},
		{
			name: "CAFile and CAPath are mutually exclusive",
			tls: &TLS{
				ClientTLS: ClientTLS{
					CAFile: certs.caFile,
				},
				CAPath: certs.caDir,
			},
			wantErr: true,
			errType: errMutuallyExclusive,
		},
		{
			name: "valid complete mTLS configuration",
			tls: &TLS{
				ClientTLS: ClientTLS{
					Name:     "tls-name",
					Keyfile:  certs.keyFile,
					Certfile: certs.certFile,
				},
			},
			wantErr: false,
		},
		{
			name: "valid mTLS with CA and password",
			tls: &TLS{
				ClientTLS: ClientTLS{
					CAFile:   certs.caFile,
					Name:     "tls-name",
					Keyfile:  certs.keyFile,
					Certfile: certs.certFile,
				},
				KeyfilePassword: "", // Empty password for unencrypted key
			},
			wantErr: false,
		},
		{
			name: "mTLS missing name",
			tls: &TLS{
				ClientTLS: ClientTLS{
					Keyfile:  certs.keyFile,
					Certfile: certs.certFile,
				},
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "mTLS missing keyfile",
			tls: &TLS{
				ClientTLS: ClientTLS{
					Name:     "tls-name",
					Certfile: certs.certFile,
				},
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "mTLS missing certfile",
			tls: &TLS{
				ClientTLS: ClientTLS{
					Name:    "tls-name",
					Keyfile: certs.keyFile,
				},
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "keyfile password without keyfile",
			tls: &TLS{
				KeyfilePassword: "password",
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "only name set should fail",
			tls: &TLS{
				ClientTLS: ClientTLS{
					Name: "tls-name",
				},
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "only keyfile set should fail",
			tls: &TLS{
				ClientTLS: ClientTLS{
					Keyfile: certs.keyFile,
				},
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "only certfile set should fail",
			tls: &TLS{
				ClientTLS: ClientTLS{
					Certfile: certs.certFile,
				},
			},
			wantErr: true,
			errType: errMissingDependency,
		},
		{
			name: "valid TLS with protocols and cipher suite",
			tls: &TLS{
				Protocols:   "TLSv1.2",
				CipherSuite: "TLS_AES_128_GCM_SHA256",
			},
			wantErr: false,
		},
		{
			name:    "unsupported TLS protocol",
			tls:     &TLS{Protocols: "TLSv1.3"},
			wantErr: true,
		},
		{
			name:    "unknown cipher suite",
			tls:     &TLS{CipherSuite: "NOT_A_CIPHER"},
			wantErr: true,
		},
		{
			name:    "empty cipher suite list",
			tls:     &TLS{CipherSuite: " : "},
			wantErr: true,
		},
		{
			name: "complex valid configuration",
			tls: &TLS{
				ClientTLS: ClientTLS{
					CAFile:   certs.caFile,
					Name:     "tls-name",
					Keyfile:  certs.keyFile,
					Certfile: certs.certFile,
				},
				Protocols:   "TLSv1.2",
				CipherSuite: "TLS_AES_128_GCM_SHA256",
			},
			wantErr: false,
		},
		{
			name: "unavailable CA file is structurally valid",
			tls: &TLS{
				ClientTLS: ClientTLS{
					CAFile: "/nonexistent/path/ca.pem",
				},
			},
			wantErr: false,
		},
		{
			name: "non-clean CA file path",
			tls: &TLS{
				ClientTLS: ClientTLS{
					CAFile: "/etc//passwd",
				},
			},
			wantErr: true,
			errType: errInvalidPath,
		},
		{
			name: "unavailable key file is structurally valid",
			tls: &TLS{
				ClientTLS: ClientTLS{
					Name:     "tls-name",
					Keyfile:  "/nonexistent/path/key.pem",
					Certfile: certs.certFile,
				},
			},
			wantErr: false,
		},
		{
			name: "unavailable cert file is structurally valid",
			tls: &TLS{
				ClientTLS: ClientTLS{
					Name:     "tls-name",
					Keyfile:  certs.keyFile,
					Certfile: "/nonexistent/path/cert.pem",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tls.Validate(ValidationDefault)

			if tt.wantErr {
				require.Error(t, err)

				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTLS_validateCACertificates(t *testing.T) {
	certs := setupTestCertificates(t)

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
				ClientTLS: ClientTLS{
					CAFile: certs.caFile,
				},
			},
			wantErr: false,
		},
		{
			name: "only CAPath",
			tls: &TLS{
				CAPath: certs.caDir,
			},
			wantErr: false,
		},
		{
			name: "both CAFile and CAPath",
			tls: &TLS{
				ClientTLS: ClientTLS{
					CAFile: certs.caFile,
				},
				CAPath: certs.caDir,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tls.validateCACertificates()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
