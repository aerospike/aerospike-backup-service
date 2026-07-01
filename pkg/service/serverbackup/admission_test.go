package serverbackup

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartController_DeniesSecondBackupOnSameCluster(t *testing.T) {
	now := time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)
	cluster := &model.AerospikeCluster{
		SeedNodes: []model.SeedNode{{HostName: "localhost", Port: 3000}},
	}
	routineOne := &model.BackupRoutine{
		Name:          "routine-one",
		SourceCluster: cluster,
	}
	routineTwo := &model.BackupRoutine{
		Name:          "routine-two",
		SourceCluster: cluster,
	}

	controller := NewStartController(NewGate())

	release, err := controller.TryStart(routineOne, now, model.BackupTypeFull)
	require.NoError(t, err)
	require.NotNil(t, release)

	secondRelease, err := controller.TryStart(routineTwo, now, model.BackupTypeFull)
	assert.Nil(t, secondRelease)
	require.ErrorIs(t, err, errBackupSkipped)
	require.ErrorIs(t, err, errAlreadyRunning)

	release()

	thirdRelease, err := controller.TryStart(routineTwo, now, model.BackupTypeFull)
	require.NoError(t, err)
	require.NotNil(t, thirdRelease)
	thirdRelease()
}

func TestStartController_AllowsDifferentClusters(t *testing.T) {
	now := time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)
	routineOne := &model.BackupRoutine{
		Name: "routine-one",
		SourceCluster: &model.AerospikeCluster{
			SeedNodes: []model.SeedNode{{HostName: "localhost", Port: 3000}},
		},
	}
	routineTwo := &model.BackupRoutine{
		Name: "routine-two",
		SourceCluster: &model.AerospikeCluster{
			SeedNodes: []model.SeedNode{{HostName: "other", Port: 3000}},
		},
	}

	controller := NewStartController(NewGate())

	releaseOne, err := controller.TryStart(routineOne, now, model.BackupTypeFull)
	require.NoError(t, err)

	releaseTwo, err := controller.TryStart(routineTwo, now, model.BackupTypeFull)
	require.NoError(t, err)

	releaseOne()
	releaseTwo()
}
