package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestBackupMetadata_BackupType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		from time.Time
		want BackupType
	}{
		{name: "lower bound is incremental", from: time.Unix(1773669600, 0), want: BackupTypeIncremental},
		{name: "no lower bound is full", want: BackupTypeFull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, BackupMetadata{From: tt.from}.BackupType())
		})
	}
}
