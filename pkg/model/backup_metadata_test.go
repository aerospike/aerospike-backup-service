package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewMetadataFromBytes_Compression(t *testing.T) {
	const base = `created: 2025-02-01T00:00:00Z
namespace: ns1
`
	tests := []struct {
		name string
		yaml string
		want CompressionMode
	}{
		{"zstd is kept", base + "compression: ZSTD\n", CompressionModeZSTD},
		{"none is kept", base + "compression: NONE\n", CompressionModeNone},
		{"missing key (written before v3.1.0) means none", base, CompressionModeNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md, err := NewMetadataFromBytes([]byte(tt.yaml))
			require.NoError(t, err)
			require.Equal(t, tt.want, md.Compression)
		})
	}
}
