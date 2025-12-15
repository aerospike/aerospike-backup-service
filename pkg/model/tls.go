package model

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// TLS represents the Aerospike cluster TLS configuration options.
type TLS struct {
	// Path to a trusted CA certificate file.
	CAFile *string
	// Path to a directory of trusted CA certificates.
	CAPath *string
	// The default TLS name used to authenticate each TLS socket connection.
	Name *string
	// TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.
	Protocols *string
	// TLS cipher selection criteria. The format is the same as OpenSSL's Cipher List Format.
	CipherSuite *string
	// Path to the key for mutual authentication (if Aerospike cluster supports it).
	Keyfile *string
	// Password to load protected TLS-keyfile (env:VAR, file:PATH, PASSWORD).
	KeyfilePassword *string
	// Path to the chain file for mutual authentication (if Aerospike Cluster supports it).
	Certfile *string
}

// String returns a string representation of the TLS.
func (tls *TLS) String() string {
	if tls == nil {
		return nilString
	}
	return fmt.Sprintf(
		"%v:%v:%v:%v:%v:%v:%v:%v",
		ptr.ValueOrZero(tls.CAFile),
		ptr.ValueOrZero(tls.CAPath),
		ptr.ValueOrZero(tls.Name),
		ptr.ValueOrZero(tls.Protocols),
		ptr.ValueOrZero(tls.CipherSuite),
		ptr.ValueOrZero(tls.Keyfile),
		ptr.ValueOrZero(tls.KeyfilePassword),
		ptr.ValueOrZero(tls.Certfile),
	)
}
