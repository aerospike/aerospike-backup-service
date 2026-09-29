package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompressionPolicy_ToLibraryPolicy(t *testing.T) {
	policy := &CompressionPolicy{
		Mode:  CompressionModeZSTD,
		Level: 3,
	}

	converted := policy.ToLibraryPolicy()

	require.NotNil(t, converted)
	assert.Equal(t, "ZSTD", converted.Mode)
	assert.Equal(t, 3, converted.Level)
}

func TestCompressionPolicy_ToLibraryPolicy_Nil(t *testing.T) {
	var policy *CompressionPolicy

	assert.Nil(t, policy.ToLibraryPolicy())
}
