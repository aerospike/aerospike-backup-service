package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompressionModeValidate(t *testing.T) {
	tests := []struct {
		name    string
		mode    CompressionMode
		wantErr bool
	}{
		{
			name: "none",
			mode: CompressionModeNone,
		},
		{
			name: "zstd",
			mode: CompressionModeZSTD,
		},
		{
			name: "empty",
			mode: "",
		},
		{
			name: "lowercase",
			mode: "zstd",
		},
		{
			name:    "unsupported",
			mode:    "GZIP",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.mode.Validate()
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCompressionModeModelConversion(t *testing.T) {
	assert.Equal(t, model.CompressionModeZSTD, CompressionModeZSTD.ToModel())
	assert.Equal(t, model.CompressionModeZSTD, CompressionMode("zstd").ToModel())
	assert.Equal(t, CompressionModeZSTD, NewCompressionModeFromModel(model.CompressionModeZSTD))
}
