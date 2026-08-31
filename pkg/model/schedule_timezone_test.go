package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupCommonConfig_GetTimezoneOrDefault(t *testing.T) {
	t.Parallel()

	assert.Equal(t, time.UTC, (*BackupCommonConfig)(nil).GetTimezoneOrDefault())
	assert.Equal(t, time.UTC, (&BackupCommonConfig{}).GetTimezoneOrDefault())

	timezone, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	config := &BackupCommonConfig{Timezone: timezone}
	assert.Same(t, timezone, config.GetTimezoneOrDefault())
}
