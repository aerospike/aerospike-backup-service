package dto

import (
	"crypto/tls"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPSServerConfigValidate(t *testing.T) {
	certs := setupTestCertificates(t)
	otherCerts := setupTestCertificates(t)

	tests := []struct {
		name    string
		config  *HTTPSServerConfig
		options ValidationOptions
		wantErr string
	}{
		{
			name: "valid config",
			config: &HTTPSServerConfig{
				CertFile: certs.certFile,
				KeyFile:  certs.keyFile,
			},
		},
		{
			name:   "disabled without certificate",
			config: &HTTPSServerConfig{Disabled: true},
		},
		{
			name: "disabled with valid certificate",
			config: &HTTPSServerConfig{
				Disabled: true,
				CertFile: certs.certFile,
				KeyFile:  certs.keyFile,
			},
		},
		{
			name: "valid mutual TLS",
			config: &HTTPSServerConfig{
				CertFile:     certs.certFile,
				KeyFile:      certs.keyFile,
				ClientCAFile: certs.caFile,
				ClientAuth:   "require-and-verify",
			},
		},
		{
			name: "skip unavailable TLS files",
			config: &HTTPSServerConfig{
				CertFile: "/not/available/server.pem",
				KeyFile:  "/not/available/server-key.pem",
			},
			options: ValidationSkipTLSFiles,
		},
		{
			name:    "missing certificate",
			config:  &HTTPSServerConfig{KeyFile: certs.keyFile},
			wantErr: "cert-file",
		},
		{
			name:    "missing key",
			config:  &HTTPSServerConfig{CertFile: certs.certFile},
			wantErr: "key-file",
		},
		{
			name: "mismatched certificate and key",
			config: &HTTPSServerConfig{
				CertFile: certs.certFile,
				KeyFile:  otherCerts.keyFile,
			},
			wantErr: "private key does not match public key",
		},
		{
			name: "disabled with mismatched certificate and key",
			config: &HTTPSServerConfig{
				Disabled: true,
				CertFile: certs.certFile,
				KeyFile:  otherCerts.keyFile,
			},
			wantErr: "private key does not match public key",
		},
		{
			name: "client authentication without CA",
			config: &HTTPSServerConfig{
				CertFile:   certs.certFile,
				KeyFile:    certs.keyFile,
				ClientAuth: "request",
			},
			wantErr: "client-ca-file",
		},
		{
			name: "invalid minimum version",
			config: &HTTPSServerConfig{
				CertFile:   certs.certFile,
				KeyFile:    certs.keyFile,
				MinVersion: "1.1",
			},
			wantErr: "min-version",
		},
		{
			name: "insecure cipher suite",
			config: &HTTPSServerConfig{
				CertFile:     certs.certFile,
				KeyFile:      certs.keyFile,
				CipherSuites: []string{"TLS_RSA_WITH_3DES_EDE_CBC_SHA"},
			},
			wantErr: "cipher-suites",
		},
		{
			name: "unknown client authentication",
			config: &HTTPSServerConfig{
				CertFile:   certs.certFile,
				KeyFile:    certs.keyFile,
				ClientAuth: "verify-if-given",
			},
			wantErr: "client authentication",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate(test.options)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func TestHTTPSServerConfigToModelAndFromModel(t *testing.T) {
	config := &HTTPSServerConfig{
		Disabled:        true,
		Address:         "127.0.0.1",
		Port:            ptr.Of(Port(9443)),
		ContextPath:     "/api",
		Timeout:         ptr.Of(int64(1000)),
		ReadTimeout:     ptr.Of(int64(2000)),
		WriteTimeout:    ptr.Of(int64(3000)),
		IdleTimeout:     ptr.Of(int64(4000)),
		CertFile:        "/cert.pem",
		KeyFile:         "/key.pem",
		KeyFilePassword: "password",
		MinVersion:      "1.3",
		CipherSuites:    []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
		ClientCAFile:    "/ca.pem",
		ClientAuth:      "require-and-verify",
	}

	modelConfig := config.ToModel()
	roundTrip := &HTTPSServerConfig{}
	roundTrip.fromModel(modelConfig)

	assert.Equal(t, config, roundTrip)
}

func TestHTTPSServerConfigCompareReportsEveryField(t *testing.T) {
	current := &HTTPSServerConfig{
		Address:         "localhost",
		Port:            ptr.Of(Port(8443)),
		Rate:            &RateLimiterConfig{Tps: ptr.Of(1)},
		ContextPath:     "/",
		Timeout:         ptr.Of(int64(1)),
		ReadTimeout:     ptr.Of(int64(2)),
		WriteTimeout:    ptr.Of(int64(3)),
		IdleTimeout:     ptr.Of(int64(4)),
		CertFile:        "cert",
		KeyFile:         "key",
		KeyFilePassword: "password",
		MinVersion:      "1.2",
		CipherSuites:    []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
		ClientCAFile:    "ca",
		ClientAuth:      "none",
	}
	other := &HTTPSServerConfig{
		Disabled:        true,
		Address:         "0.0.0.0",
		Port:            ptr.Of(Port(9443)),
		Rate:            &RateLimiterConfig{Tps: ptr.Of(2)},
		ContextPath:     "/api",
		Timeout:         ptr.Of(int64(10)),
		ReadTimeout:     ptr.Of(int64(20)),
		WriteTimeout:    ptr.Of(int64(30)),
		IdleTimeout:     ptr.Of(int64(40)),
		CertFile:        "new-cert",
		KeyFile:         "new-key",
		KeyFilePassword: "new-password",
		MinVersion:      "1.3",
		CipherSuites:    []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
		ClientCAFile:    "new-ca",
		ClientAuth:      "require-and-verify",
	}

	err := current.Compare(other)
	require.Error(t, err)
	lines := strings.Split(err.Error(), "\n")
	assert.Len(t, lines, 16)
	for _, field := range []string{
		"Disabled", "Address", "Port", "rate changes", "ContextPath", "Timeout", "ReadTimeout", "WriteTimeout",
		"IdleTimeout", "CertFile", "KeyFile", "KeyFilePassword", "MinVersion", "CipherSuites",
		"ClientCAFile", "ClientAuth",
	} {
		assert.Contains(t, err.Error(), field)
	}
}

func TestHTTPSServerConfigSecureCipherCatalog(t *testing.T) {
	for _, suite := range tls.InsecureCipherSuites() {
		_, exists := secureServerCipherSuites[suite.Name]
		assert.False(t, exists)
	}
}
