package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetadataFromBytes_JSON(t *testing.T) {
	created := time.Date(2021, 1, 2, 3, 4, 5, 0, time.UTC)
	src := BackupMetadata{
		Created:     created,
		Finished:    created.Add(time.Second),
		Namespace:   "ns1",
		RecordCount: 10,
		Compression: CompressionModeZSTD,
		Encryption:  EncryptionModeAES256,
	}

	data, err := json.Marshal(src)
	require.NoError(t, err)

	got, err := NewMetadataFromBytes(data)
	require.NoError(t, err)
	assert.Equal(t, src.Namespace, got.Namespace)
	assert.True(t, src.Created.Equal(got.Created))
	assert.Equal(t, src.Compression, got.Compression)
	assert.Equal(t, src.Encryption, got.Encryption)
}

func TestNewMetadataFromBytes_RejectsYAML(t *testing.T) {
	_, err := NewMetadataFromBytes([]byte("created: 2021-01-02T03:04:05Z\nnamespace: ns1\n"))
	require.Error(t, err)
}
