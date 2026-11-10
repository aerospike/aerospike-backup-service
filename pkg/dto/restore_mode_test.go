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

func TestValidateRestorePolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		backupMode model.BackupMode
		policy     model.RestorePolicy
		wantErr    string
	}{
		{name: "server mode with a mode", backupMode: model.BackupModeServer,
			policy: model.RestorePolicy{Mode: model.RestoreModeFuzzy}},
		{name: "server mode with options it supports", backupMode: model.BackupModeServer,
			policy: model.RestorePolicy{
				Mode:              model.RestoreModeStrict,
				Replace:           new(false),
				Namespace:         &model.RestoreNamespace{Source: "bar", Destination: "bar"},
				EncryptionPolicy:  &model.EncryptionPolicy{Mode: model.EncryptionModeNone},
				CompressionPolicy: &model.CompressionPolicy{Mode: model.CompressionModeNone},
			}},
		{name: "scan mode without a mode", backupMode: model.BackupModeScan},
		{name: "scan mode with options the server lacks", backupMode: model.BackupModeScan,
			policy: model.RestorePolicy{Replace: new(true), Bandwidth: new(int64(1))}},
		{name: "default mode without a mode"},
		{name: "server mode without a mode", backupMode: model.BackupModeServer, wantErr: `"mode" required`},
		{name: "scan mode with a mode", backupMode: model.BackupModeScan,
			policy:  model.RestorePolicy{Mode: model.RestoreModeStrict},
			wantErr: `"mode" applies only to backup-mode "server"`},
		{name: "server mode with options it lacks", backupMode: model.BackupModeServer,
			policy: model.RestorePolicy{
				Mode:              model.RestoreModeStrict,
				BinList:           []string{"bin"},
				Replace:           new(true),
				Unique:            new(true),
				NoGeneration:      new(true),
				Bandwidth:         new(int64(1)),
				EncryptionPolicy:  &model.EncryptionPolicy{Mode: model.EncryptionModeAES128},
				CompressionPolicy: &model.CompressionPolicy{Mode: model.CompressionModeZSTD},
				Namespace:         &model.RestoreNamespace{Source: "bar", Destination: "baz"},
			},
			wantErr: "restore policy sets bin-list, replace, unique, no-generation, bandwidth, encryption, " +
				`compression, namespace into another namespace, which backup-mode "server" does not support`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &model.Config{ServiceConfig: model.ServiceConfig{
				Backup: &model.BackupCommonConfig{BackupMode: tt.backupMode},
			}}

			err := validateRestorePolicy(&tt.policy, config)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}
