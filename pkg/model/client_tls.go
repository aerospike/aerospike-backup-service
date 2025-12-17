package model

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// ClientTLS represents the TLS configuration options relevant for client-side connections.
type ClientTLS struct {
	// Path to a trusted CA certificate file in PEM format.
	CAFile *string
	// TLSName used for server certificate verification (ServerName for SNI).
	Name *string
	// Path to a client certificate file for mutual TLS authentication.
	Certfile *string
	// Path to a client private key file for mutual TLS authentication.
	Keyfile *string
}

// String returns a string representation of the ClientTLS.
func (c *ClientTLS) String() string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf(
		"%v:%v:%v:%v",
		ptr.ValueOrZero(c.CAFile),
		ptr.ValueOrZero(c.Name),
		ptr.ValueOrZero(c.Certfile),
		ptr.ValueOrZero(c.Keyfile),
	)
}
