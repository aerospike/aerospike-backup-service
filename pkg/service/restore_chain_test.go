package service

import (
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// chainBackup returns a backup of the namespace. A zero from makes it a full backup.
func chainBackup(namespace string, created, from int64) model.BackupDetails {
	metadata := model.BackupMetadata{Namespace: namespace, Created: time.UnixMilli(created)}
	if from != 0 {
		metadata.From = time.UnixMilli(from)
	}

	return model.BackupDetails{BackupMetadata: metadata}
}

// TestFindRestoreChains checks that every namespace gets its own latest full backup and only
// the incremental backups after it, though the incremental backups are read once, from the
// earliest of the chosen full backups.
func TestFindRestoreChains(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	request := &model.RestoreTimestampRequest{RoutineName: "routine", Time: time.UnixMilli(5000)}

	oldFull1 := chainBackup("ns1", 1000, 0)
	full1 := chainBackup("ns1", 3000, 0)
	full2 := chainBackup("ns2", 1000, 0)
	staleIncr1 := chainBackup("ns1", 2000, 1000) // older than the full backup ns1 restores from
	incr1 := chainBackup("ns1", 4000, 3000)
	incr2a := chainBackup("ns2", 2000, 1000)
	incr2b := chainBackup("ns2", 4000, 2000)

	reader := NewMockBackupReader(ctrl)
	gomock.InOrder(
		reader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ any, filter BackupFilter) ([]model.BackupDetails, error) {
				routineFilter, ok := filter.(*RoutineFilter)
				require.True(t, ok)
				assert.Equal(t, model.BackupTypeFull, routineFilter.BackupType())
				assert.Equal(t, "routine", routineFilter.RoutineName())
				assert.Equal(t, request.Time, *routineFilter.ToTime)

				return []model.BackupDetails{oldFull1, full2, full1}, nil
			}),
		reader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ any, filter BackupFilter) ([]model.BackupDetails, error) {
				routineFilter, ok := filter.(*RoutineFilter)
				require.True(t, ok)
				assert.Equal(t, model.BackupTypeIncremental, routineFilter.BackupType())
				assert.Equal(t, full2.Created, *routineFilter.FromTime, "the earliest chosen full backup")
				assert.Equal(t, request.Time, *routineFilter.ToTime)

				return []model.BackupDetails{staleIncr1, incr2a, incr1, incr2b}, nil
			}),
	)

	chains, err := FindRestoreChains(t.Context(), reader, request)

	require.NoError(t, err)
	assert.Equal(t, map[string]RestoreChain{
		"ns1": {Full: full1, Incrementals: []model.BackupDetails{incr1}},
		"ns2": {Full: full2, Incrementals: []model.BackupDetails{incr2a, incr2b}},
	}, chains)
	assert.ElementsMatch(t, []model.BackupDetails{full1, incr1, full2, incr2a, incr2b}, AllBackups(chains))
}

func TestFindRestoreChains_Fails(t *testing.T) {
	t.Parallel()

	readErr := errors.New("storage is down")
	full := []model.BackupDetails{chainBackup("ns1", 1000, 0)}

	tests := []struct {
		name            string
		fullBackups     []model.BackupDetails
		fullErr         error
		incrementalsErr error
		wantErr         string
	}{
		{name: "no full backups", wantErr: "no full backups found"},
		{name: "full backups unreadable", fullErr: readErr, wantErr: "failed to read full backups: storage is down"},
		{
			name:            "incremental backups unreadable",
			fullBackups:     full,
			incrementalsErr: readErr,
			wantErr:         "failed to find incremental backups: storage is down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			reader := NewMockBackupReader(ctrl)
			reader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(tt.fullBackups, tt.fullErr)
			reader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).Return(nil, tt.incrementalsErr).MaxTimes(1)

			_, err := FindRestoreChains(t.Context(), reader, &model.RestoreTimestampRequest{RoutineName: "routine"})

			require.EqualError(t, err, tt.wantErr)
		})
	}
}

// A chain of differential backups keeps every incremental backup, a chain of cumulative ones
// keeps only the newest, and an incremental backup that is not newer than the full backup is
// dropped.
func TestFilterIncrementals(t *testing.T) {
	t.Parallel()

	full := chainBackup("ns", 1000, 0)

	tests := []struct {
		name         string
		incrementals []model.BackupDetails
		wantCreated  []int64
	}{
		{
			name: "differential",
			incrementals: []model.BackupDetails{
				chainBackup("ns", 3000, 2000), chainBackup("ns", 2000, 1000), chainBackup("ns", 900, 500),
			},
			wantCreated: []int64{2000, 3000},
		},
		{
			name:         "cumulative",
			incrementals: []model.BackupDetails{chainBackup("ns", 3000, 1000), chainBackup("ns", 2000, 1000)},
			wantCreated:  []int64{3000},
		},
		{
			name:         "created with the full backup",
			incrementals: []model.BackupDetails{chainBackup("ns", 1000, 500)},
			wantCreated:  []int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			created := []int64{}
			for _, incremental := range filterIncrementals(full, tt.incrementals) {
				created = append(created, incremental.Created.UnixMilli())
			}

			assert.Equal(t, tt.wantCreated, created)
		})
	}
}
