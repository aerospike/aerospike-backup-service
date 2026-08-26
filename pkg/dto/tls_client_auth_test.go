package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSClientAuthToModel(t *testing.T) {
	tests := []struct {
		name    string
		value   TLSClientAuth
		want    model.TLSClientAuth
		wantErr bool
	}{
		{
			name:  "empty defaults to none",
			value: "",
			want:  model.TLSClientAuthNone,
		},
		{
			name:  "none",
			value: TLSClientAuthNone,
			want:  model.TLSClientAuthNone,
		},
		{
			name:  "request",
			value: TLSClientAuthRequest,
			want:  model.TLSClientAuthRequest,
		},
		{
			name:  "require-and-verify",
			value: TLSClientAuthRequireAndVerify,
			want:  model.TLSClientAuthRequireAndVerify,
		},
		{
			name:    "invalid",
			value:   "verify-if-given",
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
				assert.Equal(t, test.value, NewTLSClientAuthFromModel(got))
			}
		})
	}
}
