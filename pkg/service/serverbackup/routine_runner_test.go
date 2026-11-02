package serverbackup

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRoutineRunner_ModifiedAfter(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	at := func(minutes int) time.Time { return start.Add(time.Duration(minutes) * time.Minute) }

	// The namespaces are backed up one after another: baz's backups are newer than bar's.
	backups := map[model.BackupType][]model.BackupDetails{
		model.BackupTypeFull: {
			{BackupMetadata: model.BackupMetadata{Namespace: "bar", Created: at(0)}},
			{BackupMetadata: model.BackupMetadata{Namespace: "baz", Created: at(1)}},
		},
		model.BackupTypeIncremental: {
			{BackupMetadata: model.BackupMetadata{Namespace: "bar", Created: at(10)}},
			{BackupMetadata: model.BackupMetadata{Namespace: "baz", Created: at(11)}},
		},
	}

	tests := []struct {
		name       string
		backupType model.BackupType
		incrMode   model.IncrMode
		namespace  string
		want       *time.Time
	}{
		{name: "incremental from the namespace's last backup", backupType: model.BackupTypeIncremental,
			namespace: "bar", want: new(at(10))},
		{name: "cumulative from the namespace's last full backup", backupType: model.BackupTypeIncremental,
			incrMode: model.IncrModeCumulative, namespace: "bar", want: new(at(0))},
		{name: "incremental of a namespace with no backup", backupType: model.BackupTypeIncremental,
			namespace: "foo"},
		{name: "full has no lower bound", backupType: model.BackupTypeFull, namespace: "bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			backupReader := service.NewMockBackupReader(ctrl)
			backupReader.EXPECT().GetBackups(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, filter service.BackupFilter) ([]model.BackupDetails, error) {
					return backups[filter.(*service.RoutineFilter).BackupType()], nil
				}).AnyTimes()

			routine := &model.BackupRoutine{
				Name:         "routine",
				BackupPolicy: &model.BackupPolicy{IncrMode: tt.incrMode},
			}

			got, err := (&routineRunner{backupReader: backupReader}).
				modifiedAfter(t.Context(), routine, tt.backupType, tt.namespace)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
