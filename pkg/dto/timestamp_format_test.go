package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimestampFormatToModel(t *testing.T) {
	tests := []struct {
		name    string
		value   TimestampFormat
		want    *model.TimestampFormat
		wantErr bool
	}{
		{
			name:  "empty is unset",
			value: "",
			want:  nil,
		},
		{
			name:  "iso",
			value: TimestampFormatISO,
			want:  ptr.Of(model.TimestampFormatISO),
		},
		{
			name:  "us",
			value: TimestampFormatUS,
			want:  ptr.Of(model.TimestampFormatUS),
		},
		{
			name:  "eu",
			value: TimestampFormatEU,
			want:  ptr.Of(model.TimestampFormatEU),
		},
		{
			name:    "invalid",
			value:   "UK",
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
			assert.Equal(t, test.value, NewTimestampFormatFromModel(got))
		})
	}
}
