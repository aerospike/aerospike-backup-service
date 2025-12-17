package model

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// TLS represents the Aerospike cluster TLS configuration options.
type TLS struct {
	ClientTLS
	// Path to a directory of trusted CA certificates.
	CAPath *string
	// TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.
	Protocols *string
	// TLS cipher selection criteria. The format is the same as OpenSSL's Cipher List Format.
	CipherSuite *string
	// Password to load protected TLS-keyfile (env:VAR, file:PATH, PASSWORD).
	KeyfilePassword *string
}

// String returns a string representation of the TLS.
func (tls *TLS) String() string {
	if tls == nil {
		return nilString
	}
	return fmt.Sprintf(
		"%s:%v:%v:%v:%v",
		tls.ClientTLS.String(),
		ptr.ValueOrZero(tls.CAPath),
		ptr.ValueOrZero(tls.Protocols),
		ptr.ValueOrZero(tls.CipherSuite),
		ptr.ValueOrZero(tls.KeyfilePassword),
	)
}
