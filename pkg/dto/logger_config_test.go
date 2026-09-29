package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileLoggerConfig_ValidateFilename(t *testing.T) {
	tests := []struct {
		name    string
		config  FileLoggerConfig
		wantErr bool
	}{
		{
			name: "clean filename",
			config: FileLoggerConfig{
				Filename: Path("/var/log/backup-service.log"),
			},
		},
		{
			name: "traversal filename",
			config: FileLoggerConfig{
				Filename: Path("../outside/app.log"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
