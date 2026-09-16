package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A client certificate whose only verified chain is the
// certificate itself (self-signed leaf placed in client-ca-file) is rejected when crl-file
// is set, because no issuer CRL can exist for it.
func TestCRL_RejectsSelfSignedTrustAnchorLeaf(t *testing.T) {
	pki := createTestPKI(t, 0)
	idx := &crlIndex{byRawIssuer: map[string][]*indexedCRL{}}

	err := idx.verifyClientLeaf(tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{pki.caCert}},
	}, time.Now())

	require.Error(t, err)
	require.ErrorIs(t, err, errCRLNotFound)
}
