package dto

import (
	"crypto/tls"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerHTTPSConfigValidate(t *testing.T) {
	certs := setupTestCertificates(t)
	otherCerts := setupTestCertificates(t)

	tests := []struct {
		name    string
		config  *ServerConfigHTTPS
		options ValidationOptions
		wantErr string
	}{
		{
			name: "valid config",
			config: &ServerConfigHTTPS{
				CertFile: certs.certFile,
				KeyFile:  certs.keyFile,
			},
		},
		{
			name:   "disabled without certificate",
			config: &ServerConfigHTTPS{ListenerConfig: ListenerConfig{Disabled: true}},
		},
		{
			name: "disabled with valid certificate",
			config: &ServerConfigHTTPS{
				ListenerConfig: ListenerConfig{Disabled: true},
				CertFile:       certs.certFile,
				KeyFile:        certs.keyFile,
			},
		},
		{
			name: "valid mutual TLS",
			config: &ServerConfigHTTPS{
				CertFile:     certs.certFile,
				KeyFile:      certs.keyFile,
				ClientCAFile: certs.caFile,
				ClientAuth:   "require-and-verify",
			},
		},
		{
			name: "skip unavailable TLS files",
			config: &ServerConfigHTTPS{
				CertFile: "/not/available/server.pem",
				KeyFile:  "/not/available/server-key.pem",
			},
			options: ValidationSkipTLSFiles,
		},
		{
			name:    "missing certificate",
			config:  &ServerConfigHTTPS{KeyFile: certs.keyFile},
			wantErr: "cert-file",
		},
		{
			name:    "missing key",
			config:  &ServerConfigHTTPS{CertFile: certs.certFile},
			wantErr: "key-file",
		},
		{
			name: "mismatched certificate and key",
			config: &ServerConfigHTTPS{
				CertFile: certs.certFile,
				KeyFile:  otherCerts.keyFile,
			},
			wantErr: "private key does not match public key",
		},
		{
			name: "disabled with mismatched certificate and key",
			config: &ServerConfigHTTPS{
				ListenerConfig: ListenerConfig{Disabled: true},
				CertFile:       certs.certFile,
				KeyFile:        otherCerts.keyFile,
			},
			wantErr: "private key does not match public key",
		},
		{
			name: "client authentication without CA",
			config: &ServerConfigHTTPS{
				CertFile:   certs.certFile,
				KeyFile:    certs.keyFile,
				ClientAuth: "request",
			},
			wantErr: "client-ca-file",
		},
		{
			name: "invalid minimum version",
			config: &ServerConfigHTTPS{
				CertFile:   certs.certFile,
				KeyFile:    certs.keyFile,
				MinVersion: "1.1",
			},
			wantErr: "min-version",
		},
		{
			name: "insecure cipher suite",
			config: &ServerConfigHTTPS{
				CertFile:     certs.certFile,
				KeyFile:      certs.keyFile,
				CipherSuites: []string{"TLS_RSA_WITH_3DES_EDE_CBC_SHA"},
			},
			wantErr: "cipher-suites",
		},
		{
			name: "unknown client authentication",
			config: &ServerConfigHTTPS{
				CertFile:   certs.certFile,
				KeyFile:    certs.keyFile,
				ClientAuth: "verify-if-given",
			},
			wantErr: "client authentication",
		},
		{
			name: "secret agent password reference with agent",
			config: &ServerConfigHTTPS{
				CertFile:        certs.certFile,
				KeyFile:         certs.keyFile,
				KeyFilePassword: "secrets:agent1:tls-key",
				SecretAgentConfig: SecretAgentConfig{
					SecretAgentName: "agent1",
				},
			},
		},
		{
			name: "secret agent password reference without agent",
			config: &ServerConfigHTTPS{
				CertFile:        certs.certFile,
				KeyFile:         certs.keyFile,
				KeyFilePassword: "secrets:agent1:tls-key",
			},
			wantErr: "secret agent",
		},
		{
			name: "mutually exclusive secret agent settings",
			config: &ServerConfigHTTPS{
				CertFile: certs.certFile,
				KeyFile:  certs.keyFile,
				SecretAgentConfig: SecretAgentConfig{
					SecretAgent:     &SecretAgent{Address: "localhost", ConnectionType: "tcp"},
					SecretAgentName: "agent1",
				},
			},
			wantErr: "mutually exclusive",
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

func TestServerHTTPSConfigToModelAndFromModel(t *testing.T) {
	config := &ServerConfigHTTPS{
		ListenerConfig: ListenerConfig{
			Disabled:     true,
			Address:      "127.0.0.1",
			ContextPath:  "/api",
			Timeout:      ptr.Of(int64(1000)),
			ReadTimeout:  ptr.Of(int64(2000)),
			WriteTimeout: ptr.Of(int64(3000)),
			IdleTimeout:  ptr.Of(int64(4000)),
		},
		Port:            ptr.Of(Port(9443)),
		CertFile:        "/cert.pem",
		KeyFile:         "/key.pem",
		KeyFilePassword: "password",
		MinVersion:      "1.3",
		CipherSuites:    []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
		ClientCAFile:    "/ca.pem",
		ClientAuth:      "require-and-verify",
		SecretAgentConfig: SecretAgentConfig{
			SecretAgent: &SecretAgent{
				Address:        "localhost",
				ConnectionType: ConnectionTypeTCP,
			},
		},
	}

	modelConfig := config.ToModel()
	roundTrip := &ServerConfigHTTPS{}
	roundTrip.fromModel(modelConfig)

	assert.Equal(t, config, roundTrip)
}

func TestServerHTTPSConfigCompareReportsEveryField(t *testing.T) {
	current := &ServerConfigHTTPS{
		ListenerConfig: ListenerConfig{
			Address:      "localhost",
			Rate:         &RateLimiterConfig{Tps: ptr.Of(1)},
			ContextPath:  "/",
			Timeout:      ptr.Of(int64(1)),
			ReadTimeout:  ptr.Of(int64(2)),
			WriteTimeout: ptr.Of(int64(3)),
			IdleTimeout:  ptr.Of(int64(4)),
		},
		Port:            ptr.Of(Port(8443)),
		CertFile:        "cert",
		KeyFile:         "key",
		KeyFilePassword: "password",
		MinVersion:      "1.2",
		CipherSuites:    []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
		ClientCAFile:    "ca",
		ClientAuth:      "none",
		SecretAgentConfig: SecretAgentConfig{
			SecretAgentName: "agent-a",
		},
	}
	other := &ServerConfigHTTPS{
		ListenerConfig: ListenerConfig{
			Disabled:     true,
			Address:      "0.0.0.0",
			Rate:         &RateLimiterConfig{Tps: ptr.Of(2)},
			ContextPath:  "/api",
			Timeout:      ptr.Of(int64(10)),
			ReadTimeout:  ptr.Of(int64(20)),
			WriteTimeout: ptr.Of(int64(30)),
			IdleTimeout:  ptr.Of(int64(40)),
		},
		Port:            ptr.Of(Port(9443)),
		CertFile:        "new-cert",
		KeyFile:         "new-key",
		KeyFilePassword: "new-password",
		MinVersion:      "1.3",
		CipherSuites:    []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
		ClientCAFile:    "new-ca",
		ClientAuth:      "require-and-verify",
		SecretAgentConfig: SecretAgentConfig{
			SecretAgentName: "agent-b",
		},
	}

	err := current.Compare(other)
	require.Error(t, err)
	lines := strings.Split(err.Error(), "\n")
	assert.Len(t, lines, 17)
	for _, field := range []string{
		"Disabled", "Address", "Port", "rate changes", "ContextPath", "Timeout", "ReadTimeout", "WriteTimeout",
		"IdleTimeout", "CertFile", "KeyFile", "KeyFilePassword", "MinVersion", "CipherSuites",
		"ClientCAFile", "ClientAuth", "SecretAgentName",
	} {
		assert.Contains(t, err.Error(), field)
	}
}

func TestServerHTTPSConfigSecureCipherCatalog(t *testing.T) {
	for _, suite := range tls.InsecureCipherSuites() {
		_, exists := secureServerCipherSuites[suite.Name]
		assert.False(t, exists)
	}
}

func TestServerHTTPSConfigToModel_ResolvesSecretAgentName(t *testing.T) {
	certs := setupTestCertificates(t)
	config := &Config{
		ServiceConfig: ServiceConfig{
			ServerHTTPS: &ServerConfigHTTPS{
				CertFile:        certs.certFile,
				KeyFile:         certs.keyFile,
				KeyFilePassword: "secrets:agent1:tls-key",
				SecretAgentConfig: SecretAgentConfig{
					SecretAgentName: "agent1",
				},
			},
		},
		SecretAgents: map[string]*SecretAgent{
			"agent1": {
				Address:        "localhost",
				ConnectionType: ConnectionTypeTCP,
			},
		},
	}

	require.NoError(t, config.Validate(ValidationDefault))

	modelConfig, err := config.ToModel()
	require.NoError(t, err)
	require.NotNil(t, modelConfig.ServiceConfig.ServerHTTPS)
	require.NotNil(t, modelConfig.ServiceConfig.ServerHTTPS.SecretAgent)
	assert.Equal(t, "localhost", modelConfig.ServiceConfig.ServerHTTPS.SecretAgent.Address)
}
