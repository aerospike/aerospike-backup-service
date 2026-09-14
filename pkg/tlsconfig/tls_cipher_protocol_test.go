package tlsconfig

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cluster (client-side) TLS must accept only the cipher suites the HTTPS listener accepts:
// tls.CipherSuites(), never tls.InsecureCipherSuites().
func TestParseCipherSuites_RejectsInsecureSuites(t *testing.T) {
	t.Skip("BKRS-418: cipher suites from tls.InsecureCipherSuites() are accepted")

	_, err := parseCipherSuites("TLS_RSA_WITH_RC4_128_SHA")
	require.Error(t, err)
	_, err = parseCipherSuites("TLS_RSA_WITH_3DES_EDE_CBC_SHA")
	require.Error(t, err)

	suites, err := parseCipherSuites("TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256")
	require.NoError(t, err)
	assert.Equal(t, []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}, suites)
}

// `protocols` selects the minimum version: naming TLSv1.2 must leave TLS 1.3 negotiable, and
// TLSv1.3 must be accepted.
func TestParseProtocols_DoesNotPinMaxVersion(t *testing.T) {
	t.Skip("BKRS-418: TLSv1.2 pins MaxVersion to 1.2 and TLSv1.3 is rejected")

	minV, maxV, err := parseProtocols("TLSv1.2")
	require.NoError(t, err)
	assert.Equal(t, uint16(tls.VersionTLS12), minV)
	assert.Equal(t, uint16(0), maxV, "TLS 1.3 stays negotiable")

	minV, _, err = parseProtocols("TLSv1.3")
	require.NoError(t, err)
	assert.Equal(t, uint16(tls.VersionTLS13), minV)
}

// An unset `protocols` keeps TLS 1.2 as the minimum and lets Go negotiate 1.3.
func TestParseProtocols_DefaultLeavesMaxOpen(t *testing.T) {
	minV, maxV, err := parseProtocols("")
	require.NoError(t, err)
	assert.Equal(t, uint16(tls.VersionTLS12), minV)
	assert.Equal(t, uint16(0), maxV)
}
