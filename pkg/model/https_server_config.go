package model

import (
	"time"
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

// TLSMinVersion is the minimum accepted TLS protocol version.
type TLSMinVersion string

const (
	// TLSMinVersion12 is TLS 1.2.
	TLSMinVersion12 TLSMinVersion = "1.2"
	// TLSMinVersion13 is TLS 1.3.
	TLSMinVersion13 TLSMinVersion = "1.3"
)

// HTTPSServerConfig represents the service's HTTPS server configuration.
type HTTPSServerConfig struct {
	// Disabled controls whether the HTTPS listener is disabled.
	Disabled bool
	// Address is the address to listen on.
	Address string
	// Port is the port to listen on.
	Port *Port
	// Rate is the HTTP rate limiter configuration.
	Rate *RateLimiterConfig
	// ContextPath customizes the path for API endpoints.
	ContextPath string
	// Timeout is the timeout for reading HTTP request headers.
	Timeout *time.Duration
	// ReadTimeout is the maximum duration for reading an entire request.
	ReadTimeout *time.Duration
	// WriteTimeout is the maximum duration before timing out response writes.
	WriteTimeout *time.Duration
	// IdleTimeout is the maximum time to wait for the next keep-alive request.
	IdleTimeout *time.Duration
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

// GetDisabledOrDefault returns whether the HTTPS listener is disabled.
func (s *HTTPSServerConfig) GetDisabledOrDefault() bool {
	if s == nil {
		return defaultConfig.https.Disabled
	}
	return s.Disabled
}

// GetAddressOrDefault returns the configured address or its default.
func (s *HTTPSServerConfig) GetAddressOrDefault() string {
	if s.Address != "" {
		return s.Address
	}
	return defaultConfig.https.Address
}

// GetPortOrDefault returns the configured port or its default.
func (s *HTTPSServerConfig) GetPortOrDefault() Port {
	if s.Port != nil {
		return *s.Port
	}
	return *defaultConfig.https.Port
}

// GetRateOrDefault returns the configured rate limiter or its default.
func (s *HTTPSServerConfig) GetRateOrDefault() *RateLimiterConfig {
	if s.Rate != nil {
		return s.Rate
	}
	return defaultConfig.https.Rate
}

// GetContextPathOrDefault returns the configured context path or its default.
func (s *HTTPSServerConfig) GetContextPathOrDefault() string {
	if s.ContextPath != "" {
		return s.ContextPath
	}
	return defaultConfig.https.ContextPath
}

// GetTimeoutOrDefault returns the configured header timeout or its default.
func (s *HTTPSServerConfig) GetTimeoutOrDefault() time.Duration {
	if s.Timeout != nil {
		return *s.Timeout
	}
	return *defaultConfig.https.Timeout
}

// GetReadTimeoutOrDefault returns the configured read timeout or its default.
func (s *HTTPSServerConfig) GetReadTimeoutOrDefault() time.Duration {
	if s.ReadTimeout != nil {
		return *s.ReadTimeout
	}
	return *defaultConfig.https.ReadTimeout
}

// GetWriteTimeoutOrDefault returns the configured write timeout or its default.
func (s *HTTPSServerConfig) GetWriteTimeoutOrDefault() time.Duration {
	if s.WriteTimeout != nil {
		return *s.WriteTimeout
	}
	return *defaultConfig.https.WriteTimeout
}

// GetIdleTimeoutOrDefault returns the configured idle timeout or its default.
func (s *HTTPSServerConfig) GetIdleTimeoutOrDefault() time.Duration {
	if s.IdleTimeout != nil {
		return *s.IdleTimeout
	}
	return *defaultConfig.https.IdleTimeout
}

// GetMinVersionOrDefault returns the configured minimum TLS version or its default.
func (s *HTTPSServerConfig) GetMinVersionOrDefault() TLSMinVersion {
	if s.MinVersion != "" {
		return s.MinVersion
	}
	return defaultConfig.https.MinVersion
}

// GetCipherSuitesOrDefault returns the configured cipher suites or the empty default.
func (s *HTTPSServerConfig) GetCipherSuitesOrDefault() []string {
	if s.CipherSuites != nil {
		return s.CipherSuites
	}
	return defaultConfig.https.CipherSuites
}

// GetClientAuthOrDefault returns the configured client authentication mode or its default.
func (s *HTTPSServerConfig) GetClientAuthOrDefault() TLSClientAuth {
	if s.ClientAuth != "" {
		return s.ClientAuth
	}
	return defaultConfig.https.ClientAuth
}
