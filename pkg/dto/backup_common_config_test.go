package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackupCommonConfig_BackupModeRoundTrip checks that the backup mode reaches the model and
// comes back: the service picks its backup pipeline from the model's value.
func TestBackupCommonConfig_BackupModeRoundTrip(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		dto  BackupCommonConfig
		want model.BackupMode
	}{
		"server":      {dto: BackupCommonConfig{BackupMode: BackupModeServer}, want: model.BackupModeServer},
		"scan":        {dto: BackupCommonConfig{BackupMode: BackupModeScan}, want: model.BackupModeScan},
		"unset stays": {dto: BackupCommonConfig{}, want: ""},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, tt.dto.Validate())
			m := tt.dto.ToModel()
			assert.Equal(t, tt.want, m.BackupMode)

			var back BackupCommonConfig
			back.fromModel(m)
			assert.Equal(t, tt.dto.BackupMode, back.BackupMode)
			require.NoError(t, tt.dto.Compare(&back))
		})
	}
}

func TestBackupCommonConfig_Validate_RejectsUnknownBackupMode(t *testing.T) {
	t.Parallel()

	err := (&BackupCommonConfig{BackupMode: "cloud"}).Validate()

	require.ErrorContains(t, err, "backup-mode")
}
