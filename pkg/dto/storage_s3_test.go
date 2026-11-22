package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Server-side backup sends the S3 location in info commands, so S3Storage rejects their separators
// in either backup mode.
func TestS3Storage_ValidateInfoValues(t *testing.T) {
	tests := []struct {
		name    string
		change  func(s *S3Storage)
		wantErr string
	}{
		{name: "valid", change: func(*S3Storage) {}},
		{
			name:    "semicolon in the path",
			change:  func(s *S3Storage) { s.Path = "back;ups" },
			wantErr: `path "back;ups"`,
		},
		{
			name:    "pipe in the bucket",
			change:  func(s *S3Storage) { s.Bucket = "buc|ket" },
			wantErr: `bucket "buc|ket"`,
		},
		{
			name:    "line break in the region",
			change:  func(s *S3Storage) { s.S3Region = "eu-central-1\n" },
			wantErr: `s3-region "eu-central-1\n"`,
		},
		{
			name:    "carriage return in the profile",
			change:  func(s *S3Storage) { s.S3Profile = "back\rup" },
			wantErr: `s3-profile "back\rup"`,
		},
		{
			name:    "NUL in the endpoint",
			change:  func(s *S3Storage) { s.S3EndpointOverride = "http://minio:9000\x00" },
			wantErr: `s3-endpoint-override "http://minio:9000\x00"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &S3Storage{Bucket: "bucket", S3Region: "eu-central-1", Path: "backups"}
			tt.change(s)

			err := s.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, errInvalidValue)
			require.ErrorContains(t, err, tt.wantErr+`: must not contain ";", "|", a line break or a NUL byte`)
		})
	}
}
