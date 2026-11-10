package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// filterIncrementals: differential chain keeps every incremental, cumulative chain
// keeps only the newest one; incrementals not strictly after the full are dropped.
func TestRestoreByTime_IncrementalChainSelection(t *testing.T) {
	full := model.BackupDetails{BackupMetadata: model.BackupMetadata{Created: time.UnixMilli(1000)}}
	mk := func(created, from int64) model.BackupDetails {
		return model.BackupDetails{BackupMetadata: model.BackupMetadata{
			Created: time.UnixMilli(created), From: time.UnixMilli(from)}}
	}

	// differential
	got := filterIncrementals(full, []model.BackupDetails{mk(3000, 2000), mk(2000, 1000), mk(900, 500)})
	require.Len(t, got, 2)
	assert.Equal(t, int64(2000), got[0].Created.UnixMilli())
	assert.Equal(t, int64(3000), got[1].Created.UnixMilli())

	// cumulative
	got = filterIncrementals(full, []model.BackupDetails{mk(3000, 1000), mk(2000, 1000)})
	require.Len(t, got, 1)
	assert.Equal(t, int64(3000), got[0].Created.UnixMilli())

	// incremental with Created == full.Created is not part of the chain
	got = filterIncrementals(full, []model.BackupDetails{mk(1000, 500)})
	assert.Empty(t, got)
}
