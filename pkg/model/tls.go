package model

// TLS represents the Aerospike cluster TLS configuration options.
type TLS struct {
	ClientTLS
	// Path to a directory of trusted CA certificates.
	CAPath string
	// TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.
	Protocols string
	// Colon-separated IANA TLS 1.2 cipher suite names.
	// The suite must match the certificate key type. If omitted, Go crypto/tls
	// TLS 1.2 defaults apply (ECDHE AES-GCM, ChaCha20-Poly1305, and ECDHE AES-CBC SHA).
	CipherSuite string
	// Passphrase for an encrypted TLS key file. The value is used verbatim as the decryption password.
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
