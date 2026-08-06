package serverbackup

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	servermodels "github.com/aerospike/backup-go/pkg/server/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGate_TryAcquireAndRelease(t *testing.T) {
	gate := NewGate()

	release, err := gate.TryAcquire("cluster-a")
	require.NoError(t, err)
	require.NotNil(t, release)
	assert.True(t, gate.IsActive("cluster-a"))

	release()
	assert.False(t, gate.IsActive("cluster-a"))
}

func TestGate_DeniesSecondAcquireOnSameCluster(t *testing.T) {
	gate := NewGate()

	release, err := gate.TryAcquire("cluster-a")
	require.NoError(t, err)

	secondRelease, err := gate.TryAcquire("cluster-a")
	assert.Nil(t, secondRelease)
	require.ErrorIs(t, err, errAlreadyRunning)

	release()

	thirdRelease, err := gate.TryAcquire("cluster-a")
	require.NoError(t, err)
	require.NotNil(t, thirdRelease)
	thirdRelease()
}

func TestLastBackupTime_NoBackups(t *testing.T) {
	routine := &model.BackupRoutine{Name: "r"}
	lastRun := lastBackupTime(routine, nil)
	require.True(t, lastRun.NoFullBackup())
}

func TestLastBackupTime_SingleFullBackup(t *testing.T) {
	routine := &model.BackupRoutine{Name: "r"}
	finished := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	lastRun := lastBackupTime(routine, []servermodels.Metadata{
		{
			BackupID: "300000000",
			Nodes: []servermodels.Node{
				{Finished: finished},
			},
		},
	})

	require.Equal(t, finished, *lastRun.FullBackupTime())
	require.Nil(t, lastRun.IncrementalBackupTime())
}

func TestMetadataFinishedTime_FallsBackToBackupID(t *testing.T) {
	finished := metadataFinishedTime(servermodels.Metadata{BackupID: "300000000"})
	require.Equal(t, time.Unix(300000000+citrusleafEpoch, 0), finished)
}
