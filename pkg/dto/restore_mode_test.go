package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestoreMode_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    RestoreMode
		want    model.RestoreMode
		wantErr bool
	}{
		{name: "strict", mode: "strict", want: model.RestoreModeStrict},
		{name: "fuzzy in any case", mode: "FUZZY", want: model.RestoreModeFuzzy},
		{name: "unset", mode: ""},
		{name: "unknown", mode: "cold", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (&BaseRestorePolicy{Mode: tt.mode}).Validate()
			if tt.wantErr {
				require.ErrorIs(t, err, errInvalidValue)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, tt.mode.ToModel())
		})
	}
}

func TestValidateRestoreMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		backupMode model.BackupMode
		mode       model.RestoreMode
		wantErr    error
	}{
		{name: "server mode with a mode", backupMode: model.BackupModeServer, mode: model.RestoreModeFuzzy},
		{name: "scan mode without a mode", backupMode: model.BackupModeScan},
		{name: "default mode without a mode"},
		{name: "server mode without a mode", backupMode: model.BackupModeServer, wantErr: errEmpty},
		{name: "scan mode with a mode", backupMode: model.BackupModeScan, mode: model.RestoreModeStrict,
			wantErr: errInvalidValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &model.Config{ServiceConfig: model.ServiceConfig{
				Backup: &model.BackupCommonConfig{BackupMode: tt.backupMode},
			}}

			err := validateRestoreMode(tt.mode, config)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}
