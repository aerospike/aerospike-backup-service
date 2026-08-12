package model

import "fmt"

// TLS represents the Aerospike cluster TLS configuration options.
type TLS struct {
	ClientTLS
	// Path to a directory of trusted CA certificates.
	CAPath string
	// TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.
	Protocols string
	// TLS cipher selection criteria. The format is the same as OpenSSL's Cipher List Format.
	CipherSuite string
	// Password to load protected TLS-keyfile (env:VAR, file:PATH, PASSWORD).
	KeyfilePassword string
}

// String returns a string representation of the TLS.
func (tls *TLS) String() string {
	if tls == nil {
		return ""
	}
	return fmt.Sprintf(
		"%s:%s:%s:%s:%s",
		tls.ClientTLS.String(),
		tls.CAPath,
		tls.Protocols,
		tls.CipherSuite,
		tls.KeyfilePassword,
	)
}
