package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// When one namespace of a multi-namespace routine fails while another is still writing, nothing is
// deleted: the timestamp folder <routine>/backup/<ts> is shared by every namespace of the run, and
// the failed namespace's own folder has no metadata, so it stays out of the catalog. The sibling
// finishes and writes its metadata untouched. The mock writer fails the test on any Delete.
func TestNamespaceBackupRunner_FailureDeletesNothing(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	executor := backupexecutor.NewMockBackup(ctrl)
	writer := NewMockBackupWriter(ctrl)
	nsRunner := NewNamespaceBackupRunner(executor, writer, NewPathService(nil))

	start := time.UnixMilli(1700000000000)
	folderA := "multi/backup/1700000000000/data/nsA"
	folderB := "multi/backup/1700000000000/data/nsB"

	routine := &model.BackupRoutine{
		Name:          "multi",
		SourceCluster: &model.AerospikeCluster{},
		BackupPolicy: &model.BackupPolicy{
			RetryPolicy: &model.RetryPolicy{MaxRetries: optional.Of(0)},
		},
	}

	// nsB is a long-running backup that is still in flight when nsA fails.
	bStillRunning := make(chan struct{})
	aFailed := make(chan struct{})
	handlerB := backupexecutor.NewMockBackupHandler(ctrl)
	statsB := models.NewBackupStats()
	statsB.TotalRecords.Store(10)
	statsB.ReadRecords.Add(10)
	handlerB.EXPECT().GetStats().Return(statsB).AnyTimes()
	handlerB.EXPECT().Wait(gomock.Any()).DoAndReturn(func(context.Context) error {
		close(bStillRunning)
		<-aFailed // keep "writing" until nsA has failed

		return nil
	})
	executor.EXPECT().
		Run(gomock.Any(), routine, gomock.Any(), "nsB", folderB, gomock.Any(), gomock.Any()).
		Return(handlerB, nil)

	// nsA fails as soon as nsB is known to be running.
	handlerA := backupexecutor.NewMockBackupHandler(ctrl)
	handlerA.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()
	handlerA.EXPECT().Wait(gomock.Any()).DoAndReturn(func(context.Context) error {
		<-bStillRunning
		close(aFailed)

		return errors.New("nsA scan failed")
	})
	executor.EXPECT().
		Run(gomock.Any(), routine, gomock.Any(), "nsA", folderA, gomock.Any(), gomock.Any()).
		Return(handlerA, nil)

	var (
		mu      sync.Mutex
		written []string
	)
	writer.EXPECT().WriteBackupMetadata(gomock.Any(), routine, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ *model.BackupRoutine, folder string, _ model.BackupMetadata) error {
			mu.Lock()
			written = append(written, folder)
			mu.Unlock()

			return nil
		})

	spec := model.BackupRunSpec{Type: model.BackupTypeFull, StartTime: start}
	hA, err := nsRunner.Run(t.Context(), routine, "nsA", spec, nil, slog.Default())
	require.NoError(t, err)
	hB, err := nsRunner.Run(t.Context(), routine, "nsB", spec, nil, slog.Default())
	require.NoError(t, err)

	require.Error(t, hA.Wait(t.Context()))
	require.NoError(t, hB.Wait(t.Context()))

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{folderB}, written, "the sibling namespace completes untouched")
}
