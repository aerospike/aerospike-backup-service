package model

import (
	"crypto/tls"
	"fmt"
)

// TLSClientAuth identifies how the HTTPS listener authenticates client certificates.
type TLSClientAuth string

const (
	// TLSClientAuthNone does not request client certificates.
	TLSClientAuthNone TLSClientAuth = "none"
	// TLSClientAuthRequest requests a client certificate but does not require or verify it.
	TLSClientAuthRequest TLSClientAuth = "request"
	// TLSClientAuthRequireAndVerify requires and verifies a client certificate.
	TLSClientAuthRequireAndVerify TLSClientAuth = "require-and-verify"
)

// ToTLS maps the configured mode to crypto/tls.
func (a TLSClientAuth) ToTLS() (tls.ClientAuthType, error) {
	switch a {
	case "", TLSClientAuthNone:
		return tls.NoClientCert, nil
	case TLSClientAuthRequest:
		return tls.RequestClientCert, nil
	case TLSClientAuthRequireAndVerify:
		return tls.RequireAndVerifyClientCert, nil
	default:
		return tls.NoClientCert, fmt.Errorf("unsupported TLS client authentication mode %q", a)
	}
}

// TLSMinVersion is the minimum accepted TLS protocol version.
type TLSMinVersion string

const (
	// TLSMinVersion12 is TLS 1.2.
	TLSMinVersion12 TLSMinVersion = "1.2"
	// TLSMinVersion13 is TLS 1.3.
	TLSMinVersion13 TLSMinVersion = "1.3"
)

// ServerConfigHTTPS represents the service's HTTPS server configuration.
type ServerConfigHTTPS struct {
	ListenerConfig
	// The port to listen on.
	Port *Port
	// CertFile is the path to the server certificate.
	CertFile string
	// KeyFile is the path to the server private key.
	KeyFile string
	// KeyFilePassword is the passphrase for an encrypted server private key.
	// This may be a literal value or a Secret Agent reference.
	KeyFilePassword string
	// SecretAgent is used to resolve KeyFilePassword when it is a Secret Agent reference.
	SecretAgent *SecretAgent
	// MinVersion is the minimum accepted TLS protocol version.
	MinVersion TLSMinVersion
	// CipherSuites is the optional list of allowed TLS cipher suite names.
	CipherSuites []string
	// ClientCAFile is the path to trusted client CA certificates.
	ClientCAFile string
	// ClientAuth controls client certificate authentication.
	ClientAuth TLSClientAuth
}

// GetPortOrDefault returns the configured port or its default.
func (s *ServerConfigHTTPS) GetPortOrDefault() Port {
	if s == nil || s.Port == nil {
		return *defaultConfig.https.Port
	}
	return *s.Port
}

// GetMinVersionOrDefault returns the configured minimum TLS version or its default.
func (s *ServerConfigHTTPS) GetMinVersionOrDefault() TLSMinVersion {
	if s == nil || s.MinVersion == "" {
		return defaultConfig.https.MinVersion
	}
	return s.MinVersion
}

// GetCipherSuitesOrDefault returns the configured cipher suites or the empty default.
func (s *ServerConfigHTTPS) GetCipherSuitesOrDefault() []string {
	if s == nil || s.CipherSuites == nil {
		return defaultConfig.https.CipherSuites
	}
	return s.CipherSuites
}

// GetClientAuthOrDefault returns the configured client authentication mode or its default.
func (s *ServerConfigHTTPS) GetClientAuthOrDefault() TLSClientAuth {
	if s == nil || s.ClientAuth == "" {
		return defaultConfig.https.ClientAuth
	}
	return s.ClientAuth
}
