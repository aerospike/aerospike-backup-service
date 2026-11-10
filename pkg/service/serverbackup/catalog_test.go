package serverbackup

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	listermodels "github.com/aerospike/backup-go/pkg/server/lister/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToBackupDetails(t *testing.T) {
	t.Parallel()

	storage := &model.S3Storage{Bucket: "bucket", Path: "backups"}
	jobTime := time.Date(2026, 10, 20, 9, 1, 11, 0, time.UTC)
	runStart := jobTime.Add(-time.Hour)

	tests := []struct {
		name        string
		metadata    listermodels.Metadata
		wantCreated time.Time
		wantFrom    time.Time
	}{
		{
			name:        "unsealed full is as of its job start",
			metadata:    listermodels.Metadata{BackupID: fullID, Namespace: "bar"},
			wantCreated: jobTime,
		},
		{
			// A namespace backed up after others starts its job well after the run: its data is
			// still as of the run's start, the next incremental's lower bound.
			name:        "sealed is as of its upper bound",
			metadata:    listermodels.Metadata{BackupID: fullID, Namespace: "bar", ModifiedBeforeUnixS: runStart.Unix()},
			wantCreated: runStart,
		},
		{
			name: "incremental",
			metadata: listermodels.Metadata{
				BackupID: fullID, Namespace: "bar", ModifiedAfterUnixS: runStart.Add(-time.Hour).Unix(),
			},
			wantCreated: jobTime,
			wantFrom:    runStart.Add(-time.Hour),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := toBackupDetails(tt.metadata, storage, "routine")

			assert.Equal(t, "routine/"+fullID, got.Key)
			assert.Equal(t, "bar", got.Namespace)
			assert.Equal(t, tt.wantCreated, got.Created)
			assert.Equal(t, time.UTC, got.Created.Location(), "times compare equal as map keys")
			assert.Equal(t, tt.wantFrom, got.From)
			assert.Equal(t, uint64(1), got.FileCount, "restore skips backups with no files")
		})
	}
}

func TestParseBackupKey(t *testing.T) {
	t.Parallel()

	routineName, jobID, err := parseBackupKey(backupKey("routine", fullID))
	require.NoError(t, err)
	assert.Equal(t, "routine", routineName)
	assert.Equal(t, fullID, jobID)

	for _, key := range []string{fullID, "/" + fullID} {
		_, _, err := parseBackupKey(key)
		require.Error(t, err, key)
	}
}

func TestRoutinePath(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "backups/routine", routinePath(&model.S3Storage{Path: "backups"}, "routine"))
	assert.Equal(t, "routine", routinePath(&model.S3Storage{}, "routine"), "a storage at the bucket root")
}
