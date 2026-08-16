package model

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

// Hash returns a unique identifier for the TLS configuration.
func (tls *TLS) Hash() uint64 {
	if tls == nil {
		return 0
	}

	return hashValues(
		tls.ClientTLS.Hash(),
		tls.CAPath,
		tls.Protocols,
		tls.CipherSuite,
		tls.KeyfilePassword,
	)
}
