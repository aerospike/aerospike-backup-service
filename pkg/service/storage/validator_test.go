package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNameValidator(t *testing.T) {
	t.Parallel()

	assert.Nil(t, newNameValidator(""))

	v := newNameValidator(".asb")
	require.NotNil(t, v)
	assert.Equal(t, ".asb", v.filter)
}

func TestNameValidator_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		v       *nameValidator
		path    string
		wantErr bool
	}{
		{name: "nil validator allows everything", v: nil, path: "backup/data.asb", wantErr: false},
		{name: "empty filter allows everything", v: &nameValidator{filter: ""}, path: "backup/data.asb", wantErr: false},
		{name: "matching suffix", v: &nameValidator{filter: ".asb"}, path: "backup/data.asb", wantErr: false},
		{name: "non-matching suffix", v: &nameValidator{filter: ".asb"}, path: "backup/data.txt", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.v.Run(tt.path)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "skipped by filter")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
