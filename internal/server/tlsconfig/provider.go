package tlsconfig

import "crypto/tls"

// DynamicTLS supplies certificate and client-CA material for live HTTPS handshakes.
type DynamicTLS interface {
	// GetCertificate returns the server key pair for a TLS handshake.
	GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error)
	// GetConfigForClient returns a per-handshake tls.Config clone with the current client CA pool.
	GetConfigForClient(*tls.ClientHelloInfo) (*tls.Config, error)
	// SetBaseConfig stores the listener tls.Config template used by GetConfigForClient.
	SetBaseConfig(*tls.Config)
}
