package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The incremental-cron validation error quotes the incremental cron, not the full one.
func TestBackupRoutineValidate_IncrCronErrorQuotesIncrementalCron(t *testing.T) {
	r := &BackupRoutine{
		SourceCluster:    "c",
		Storage:          "s",
		IntervalCron:     "0 0 * * * *",
		IncrIntervalCron: "not-a-cron",
		Namespaces:       &[]string{},
	}

	err := r.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incremental backup interval string 'not-a-cron' invalid")
}
