package model

// ClientTLS represents the TLS configuration options relevant for client-side connections.
type ClientTLS struct {
	// Path to a trusted CA certificate file in PEM format.
	CAFile string
	// TLSName used for server certificate verification (ServerName for SNI).
	Name string
	// Path to a client certificate file for mutual TLS authentication.
	Certfile string
	// Path to a client private key file for mutual TLS authentication.
	Keyfile string
}

// Hash returns a unique identifier for the ClientTLS configuration.
func (c ClientTLS) Hash() uint64 {
	return hashValues(c.CAFile, c.Name, c.Certfile, c.Keyfile)
}
