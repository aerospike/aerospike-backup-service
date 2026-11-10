package serverbackup

import (
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStartController_TryStart(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	cluster := &model.AerospikeCluster{ClusterLabel: "a"}
	first := &model.BackupRoutine{Name: "first", SourceCluster: cluster}
	second := &model.BackupRoutine{Name: "second", SourceCluster: cluster}
	elsewhere := &model.BackupRoutine{Name: "elsewhere", SourceCluster: &model.AerospikeCluster{ClusterLabel: "b"}}

	t.Run("routines on other clusters run at once", func(t *testing.T) {
		t.Parallel()
		routineGate := admittingGate(t, 2)

		c := NewStartController(routineGate)
		_, err := c.TryStart(first, now, model.BackupTypeFull)
		require.NoError(t, err)
		_, err = c.TryStart(elsewhere, now, model.BackupTypeFull)
		require.NoError(t, err)
	})

	t.Run("a second routine on the cluster is skipped", func(t *testing.T) {
		t.Parallel()
		routineGate := service.NewMockStartController(gomock.NewController(t))
		released := 0
		routineGate.EXPECT().TryStart(gomock.Any(), now, model.BackupTypeFull).
			Return(func() { released++ }, nil).Times(2)

		c := NewStartController(routineGate)
		_, err := c.TryStart(first, now, model.BackupTypeFull)
		require.NoError(t, err)

		_, err = c.TryStart(second, now, model.BackupTypeFull)
		require.ErrorIs(t, err, service.ErrBackupSkipped)
		assert.Equal(t, 1, released, "the skipped routine's own reservation is released")
	})

	t.Run("the routine's rules apply first", func(t *testing.T) {
		t.Parallel()
		denied := errors.Join(service.ErrBackupSkipped, errors.New("no full backup yet"))
		routineGate := service.NewMockStartController(gomock.NewController(t))
		gomock.InOrder(
			routineGate.EXPECT().TryStart(first, now, model.BackupTypeIncremental).Return(nil, denied),
			routineGate.EXPECT().TryStart(second, now, model.BackupTypeFull).Return(func() {}, nil),
		)

		c := NewStartController(routineGate)
		_, err := c.TryStart(first, now, model.BackupTypeIncremental)
		require.ErrorIs(t, err, denied)

		_, err = c.TryStart(second, now, model.BackupTypeFull)
		require.NoError(t, err, "a denied start holds no cluster")
	})

	t.Run("release frees the cluster and the routine once", func(t *testing.T) {
		t.Parallel()
		routineGate := service.NewMockStartController(gomock.NewController(t))
		released := 0
		routineGate.EXPECT().TryStart(gomock.Any(), now, model.BackupTypeFull).
			Return(func() { released++ }, nil).Times(2)

		c := NewStartController(routineGate)
		release, err := c.TryStart(first, now, model.BackupTypeFull)
		require.NoError(t, err)
		release()
		release()
		assert.Equal(t, 1, released)

		_, err = c.TryStart(second, now, model.BackupTypeFull)
		require.NoError(t, err)
	})
}

// TestStartController_HasBackupRunning checks that a routine runs a backup only when it holds
// one itself, not when another routine holds its cluster.
func TestStartController_HasBackupRunning(t *testing.T) {
	t.Parallel()
	routine := &model.BackupRoutine{Name: "routine", SourceCluster: &model.AerospikeCluster{}}

	routineGate := service.NewMockStartController(gomock.NewController(t))
	routineGate.EXPECT().HasBackupRunning(routine).Return(true)

	assert.True(t, NewStartController(routineGate).HasBackupRunning(routine))
}

// admittingGate returns a routine gate that admits times starts.
func admittingGate(t *testing.T, times int) service.StartController {
	t.Helper()

	routineGate := service.NewMockStartController(gomock.NewController(t))
	routineGate.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil).Times(times)

	return routineGate
}
