package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSClientAuthValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   TLSClientAuth
		wantErr bool
	}{
		{
			name:  "none",
			value: TLSClientAuthNone,
		},
		{
			name:  "empty",
			value: "",
		},
		{
			name:  "mixed case",
			value: "Require-And-Verify",
		},
		{
			name:    "invalid",
			value:   "verify-if-given",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.value.Validate()
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestTLSClientAuthToModel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, model.TLSClientAuth(""), TLSClientAuth("").ToModel())
	assert.Equal(t, model.TLSClientAuthNone, TLSClientAuthNone.ToModel())
	assert.Equal(t, model.TLSClientAuthRequest, TLSClientAuthRequest.ToModel())
	assert.Equal(t, model.TLSClientAuthRequireAndVerify, TLSClientAuthRequireAndVerify.ToModel())
	assert.Equal(t, model.TLSClientAuthRequireAndVerify, TLSClientAuth("Require-And-Verify").ToModel())
	assert.Equal(t, TLSClientAuthRequireAndVerify, NewTLSClientAuthFromModel(model.TLSClientAuthRequireAndVerify))
}
