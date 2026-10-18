package tlsconfig

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An unset `protocols` keeps TLS 1.2 as the minimum and lets Go negotiate 1.3.
func TestParseProtocols_DefaultLeavesMaxOpen(t *testing.T) {
	minV, maxV, err := parseProtocols("")
	require.NoError(t, err)
	assert.Equal(t, uint16(tls.VersionTLS12), minV)
	assert.Equal(t, uint16(0), maxV)
}
