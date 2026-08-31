package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSMinVersionToModel(t *testing.T) {
	tests := []struct {
		name    string
		value   TLSMinVersion
		want    model.TLSMinVersion
		wantErr bool
	}{
		{
			name:  "empty defaults to 1.2",
			value: "",
			want:  model.TLSMinVersion12,
		},
		{
			name:  "1.2",
			value: TLSMinVersion12,
			want:  model.TLSMinVersion12,
		},
		{
			name:  "1.3",
			value: TLSMinVersion13,
			want:  model.TLSMinVersion13,
		},
		{
			name:    "invalid",
			value:   "1.1",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.value.ToModel()
			if test.wantErr {
				require.Error(t, err)
				require.Error(t, test.value.Validate())
				return
			}
			require.NoError(t, err)
			require.NoError(t, test.value.Validate())
			assert.Equal(t, test.want, got)
			if test.value != "" {
				assert.Equal(t, test.value, NewTLSMinVersionFromModel(got))
			}
		})
	}
}
