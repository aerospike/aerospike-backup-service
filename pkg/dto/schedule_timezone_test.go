package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseScheduleTimezone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    *time.Location
		wantErr string
	}{
		{
			name:  "empty defaults to UTC",
			input: "",
			want:  time.UTC,
		},
		{
			name:  "utc lowercase",
			input: "utc",
			want:  time.UTC,
		},
		{
			name:  "UTC uppercase",
			input: "UTC",
			want:  time.UTC,
		},
		{
			name:  "local",
			input: "local",
			want:  time.Local,
		},
		{
			name:  "Local mixed case",
			input: "Local",
			want:  time.Local,
		},
		{
			name:  "IANA America/New_York",
			input: "America/New_York",
		},
		{
			name:    "EST abbreviation rejected",
			input:   "EST",
			wantErr: "EST",
		},
		{
			name:    "unknown IANA name",
			input:   "Not/AZone",
			wantErr: "Not/AZone",
		},
		{
			name:    "POSIX TZ string rejected",
			input:   "EST5EDT",
			wantErr: "EST5EDT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseTimezone(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			if tt.want != nil {
				assert.Equal(t, tt.want, got)
				return
			}

			expected, err := time.LoadLocation(tt.input)
			require.NoError(t, err)
			assert.Equal(t, expected, got)
		})
	}
}
