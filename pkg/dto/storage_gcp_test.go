package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGcpStorage_ValidateKeyFilePath(t *testing.T) {
	tests := []struct {
		name    string
		storage GcpStorage
		wantErr bool
	}{
		{
			name: "ok",
			storage: GcpStorage{
				BucketName: "bucket",
				KeyFile:    Path("service-account.json"),
			},
		},
		{
			name: "clean key file path",
			storage: GcpStorage{
				BucketName: "bucket",
				KeyFile:    Path("/etc/gcp/service-account.json"),
			},
		},
		{
			name: "traversal key file path",
			storage: GcpStorage{
				BucketName: "bucket",
				KeyFile:    Path("../outside/service-account.json"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.storage.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
