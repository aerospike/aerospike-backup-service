package dto

import (
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStorageFromReader(t *testing.T) {
	t.Parallel()

	jsonStorage := `{"local-storage": {"path": "backups"}}`
	s, err := NewStorageFromReader(strings.NewReader(jsonStorage), decoder.JSON)
	require.NoError(t, err)
	require.NotNil(t, s.LocalStorage)
	assert.Equal(t, "backups", s.LocalStorage.Path)
}

func TestNewStorageFromReader_ValidationError(t *testing.T) {
	t.Parallel()

	// No storage type specified: passes deserialization but fails Validate.
	_, err := NewStorageFromReader(strings.NewReader(`{}`), decoder.JSON)
	require.Error(t, err)
}

func TestNewStorageFromReader_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := NewStorageFromReader(strings.NewReader(`{"unknown-field": 1}`), decoder.JSON)
	require.Error(t, err)
}
