package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBackupStarter_LowerBounds(t *testing.T) {
	t.Parallel()

	last := map[string]time.Time{"bar": time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)}

	tests := []struct {
		name       string
		backupType model.BackupType
		incrMode   model.IncrMode
		wantTypes  []model.BackupType
	}{
		{name: "incremental from the namespace's last backup of either type",
			backupType: model.BackupTypeIncremental,
			wantTypes:  []model.BackupType{model.BackupTypeFull, model.BackupTypeIncremental}},
		{name: "cumulative from the namespace's last full backup", backupType: model.BackupTypeIncremental,
			incrMode: model.IncrModeCumulative, wantTypes: []model.BackupType{model.BackupTypeFull}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			routine := &model.BackupRoutine{
				Name:         "routine",
				BackupPolicy: &model.BackupPolicy{IncrMode: tt.incrMode},
			}
			catalog := NewMockCatalog(gomock.NewController(t))
			catalog.EXPECT().LastBackupTimes(gomock.Any(), routine, tt.wantTypes).Return(last, nil)

			bounds, err := (&backupStarter{catalog: catalog}).lowerBounds(t.Context(), routine, tt.backupType)

			require.NoError(t, err)
			assert.Equal(t, last, bounds)
		})
	}
}

func TestBackupStarter_LowerBounds_Full(t *testing.T) {
	t.Parallel()

	routine := &model.BackupRoutine{Name: "routine", BackupPolicy: &model.BackupPolicy{}}

	// The mock fails the test on any call: a full backup reads no storage.
	bounds, err := (&backupStarter{catalog: NewMockCatalog(gomock.NewController(t))}).
		lowerBounds(t.Context(), routine, model.BackupTypeFull)

	require.NoError(t, err)
	assert.Nil(t, bounds)
	assert.Nil(t, lowerBound(bounds, "bar"))
}

func TestBackupStarter_LowerBounds_ReadFails(t *testing.T) {
	t.Parallel()

	catalog := NewMockCatalog(gomock.NewController(t))
	catalog.EXPECT().LastBackupTimes(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("storage down"))
	routine := &model.BackupRoutine{Name: "routine", BackupPolicy: &model.BackupPolicy{}}

	got, err := (&backupStarter{catalog: catalog}).lowerBounds(t.Context(), routine, model.BackupTypeIncremental)

	require.EqualError(t, err, "failed to read the last backups of routine routine: storage down")
	assert.Nil(t, got)
}

func TestBackupRequest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 28, 12, 30, 0, 0, time.UTC)
	lastBackup := now.Add(-20 * time.Minute)
	unix := func(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }

	storage := &model.S3Storage{Bucket: "bucket", S3Region: "eu-central-1", Path: "backups", S3Profile: "backup"}
	common := infomodels.RequestCommon{
		Namespace: "bar", Storage: "aws-s3", Path: "backups/routine", Bucket: "bucket",
		Region: "eu-central-1", Profile: "backup",
	}

	tests := []struct {
		name          string
		routine       *model.BackupRoutine
		backupType    model.BackupType
		modifiedAfter *time.Time
		want          infomodels.RequestBackup
	}{
		{
			name: "full with every option",
			routine: &model.BackupRoutine{
				Name:             "routine",
				Storage:          storage,
				SetList:          []string{"setA", "setB"},
				BinList:          []string{"bin1", "bin2"},
				FilterExpression: "kxkCkwECAA==",
				BackupPolicy: &model.BackupPolicy{
					RecordsPerSecond:  new(100),
					Parallel:          new(2),
					ParallelWrite:     new(3),
					NoIndexes:         new(true),
					NoUdfs:            new(false),
					WithClusterConfig: new(true),
					Sealed:            new(true),
				},
			},
			backupType: model.BackupTypeFull,
			want: infomodels.RequestBackup{
				RequestCommon: func() infomodels.RequestCommon {
					c := common
					c.SetList = "setA,setB"
					c.FilterExp = "kxkCkwECAA=="
					c.NoIndexes = new(true)
					c.NoUDFs = new(false)

					return c
				}(),
				NoConfig: new(false),
				BinList:  "bin1,bin2",
				// A sealed backup is bounded by the time it started.
				ModifiedBefore:      unix(now),
				BackupRPS:           100,
				BackupWorkerThreads: 2,
				BackupUploadThreads: 3,
			},
		},
		{
			// An unset limit is left to the server.
			name:          "incremental",
			routine:       &model.BackupRoutine{Name: "routine", Storage: storage, BackupPolicy: &model.BackupPolicy{}},
			backupType:    model.BackupTypeIncremental,
			modifiedAfter: &lastBackup,
			want:          infomodels.RequestBackup{RequestCommon: common, NoConfig: new(true), ModifiedAfter: unix(lastBackup)},
		},
		{
			name:       "full without the cluster configuration",
			routine:    &model.BackupRoutine{Name: "routine", Storage: storage, BackupPolicy: &model.BackupPolicy{}},
			backupType: model.BackupTypeFull,
			want:       infomodels.RequestBackup{RequestCommon: common, NoConfig: new(true)},
		},
		{
			// Only a full backup stores the cluster configuration.
			name: "incremental with the cluster configuration",
			routine: &model.BackupRoutine{
				Name: "routine", Storage: storage,
				BackupPolicy: &model.BackupPolicy{WithClusterConfig: new(true)},
			},
			backupType:    model.BackupTypeIncremental,
			modifiedAfter: &lastBackup,
			want:          infomodels.RequestBackup{RequestCommon: common, NoConfig: new(true), ModifiedAfter: unix(lastBackup)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request, err := backupRequest(tt.routine, tt.backupType, now, tt.modifiedAfter, "bar")

			require.NoError(t, err)
			assert.Equal(t, &tt.want, request)
		})
	}
}

func TestBackupRequest_NotS3(t *testing.T) {
	t.Parallel()

	routine := &model.BackupRoutine{Storage: &model.LocalStorage{Path: "/backups"}, BackupPolicy: &model.BackupPolicy{}}

	_, err := backupRequest(routine, model.BackupTypeFull, time.Now(), nil, "bar")

	require.EqualError(t, err, "server backup requires S3 storage, got *model.LocalStorage")
}

// TestBackupStarter_StartJobs checks that a start begins the job of each namespace from where the
// namespace's last backup ended, reads the storage once for all of them, and returns the jobs in
// the namespaces' order.
func TestBackupStarter_StartJobs(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	routine := testRoutine("daily")
	lastBackups := map[string]time.Time{"bar": time.Unix(1763980000, 0), "baz": time.Unix(1763970000, 0)}
	catalog := NewMockCatalog(ctrl)
	catalog.EXPECT().LastBackupTimes(gomock.Any(), routine, gomock.Any()).Return(lastBackups, nil)
	var mu sync.Mutex // the namespaces' jobs start at once.
	modifiedAfter := make(map[string]string)
	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().StartBackup(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *infomodels.RequestBackup) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			modifiedAfter[request.Namespace] = request.ModifiedAfter

			return "job-" + request.Namespace, nil
		}).Times(2)
	starter := newTestStarter(ctrl, routine, infoClient, []string{"bar", "baz"}, nil)
	starter.catalog = catalog

	jobs, err := starter.startJobs(t.Context(), routine, model.BackupTypeIncremental, time.Now(), discardLogger)

	require.NoError(t, err)
	assert.Equal(t, []startedJob{{namespace: "bar", id: "job-bar"}, {namespace: "baz", id: "job-baz"}}, jobs)
	assert.Equal(t, map[string]string{"bar": "1763980000", "baz": "1763970000"}, modifiedAfter)
}

// TestBackupStarter_StartJobs_Failures checks that a namespace whose job does not start does not
// stop the others, and that a job whose start has no known outcome counts as started: the
// listings tell whether it runs, and nothing aborts it.
func TestBackupStarter_StartJobs_Failures(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	routine := testRoutine("daily")
	// The mock fails the test on an abort: none is expected.
	infoClient := aerospike.NewMockInfoGetter(ctrl)
	infoClient.EXPECT().StartBackup(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *infomodels.RequestBackup) (string, error) {
			switch request.Namespace {
			case "bar":
				return "job-bar", errors.New("refused")
			case "baz":
				return "job-baz", fmt.Errorf("%w: timeout", asinfo.ErrOutcomeUnknown)
			default:
				return "job-" + request.Namespace, nil
			}
		}).Times(3)
	starter := newTestStarter(ctrl, routine, infoClient, []string{"bar", "baz", "qux"}, nil)

	jobs, err := starter.startJobs(t.Context(), routine, model.BackupTypeFull, time.Now(), discardLogger)

	require.EqualError(t, err, "namespace bar: failed to start server backup: refused")
	assert.Equal(t, []startedJob{{namespace: "baz", id: "job-baz"}, {namespace: "qux", id: "job-qux"}}, jobs)
}

// TestBackupStarter_Backup checks what a backup leaves: a run that holds its jobs, or, when it
// starts no job, its failure reported at once. A backup the start rules deny is reported skipped.
// The reservation is released once the start ends.
func TestBackupStarter_Backup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		denied     error
		resolveErr error
		startErr   error
		wantReport string
		wantJobs   bool
	}{
		{name: "started", wantJobs: true},
		{
			name:       "denied",
			denied:     fmt.Errorf("%w: full backup already in progress", service.ErrBackupSkipped),
			wantReport: "backup skipped: full backup already in progress",
		},
		{
			name:       "no namespace resolved",
			resolveErr: errors.New("cluster down"),
			wantReport: "full backup failed: cluster down",
		},
		{
			name:       "no job started",
			startErr:   errors.New("refused"),
			wantReport: "full backup failed: namespace bar: failed to start server backup: refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			routine := testRoutine("daily")
			runs := NewRuns()
			// The cluster has been listed: the start does not wait.
			runs.update(routine.SourceCluster, nil, time.Now(), testRoutines{})
			now := time.Now()

			released := 0
			startController := service.NewMockStartController(ctrl)
			if tt.denied != nil {
				startController.EXPECT().TryStart(routine, now, model.BackupTypeFull).Return(nil, tt.denied)
			} else {
				startController.EXPECT().TryStart(routine, now, model.BackupTypeFull).
					Return(func() { released++ }, nil)
			}
			reporter := service.NewMockBackupReporter(ctrl)
			if tt.wantReport != "" {
				reporter.EXPECT().Report(routine.Name, model.BackupTypeFull, gomock.Any(), gomock.Any(), gomock.Any()).
					Do(func(_ string, _ model.BackupType, _ time.Duration, err error, _ any) {
						assert.EqualError(t, err, tt.wantReport)
					})
			}
			starter := &backupStarter{startController: startController, reporter: reporter, runs: runs}
			namespaceResolver := aerospike.NewMockNamespaceResolver(ctrl)
			namespaceResolver.EXPECT().ResolveNamespaces(gomock.Any(), routine, gomock.Any()).
				Return([]string{"bar"}, tt.resolveErr).MaxTimes(1)
			starter.namespaceResolver = namespaceResolver
			if tt.denied == nil && tt.resolveErr == nil {
				infoClient := aerospike.NewMockInfoGetter(ctrl)
				infoClient.EXPECT().StartBackup(gomock.Any(), gomock.Any()).Return("job-bar", tt.startErr)
				starter.clientManager = newTestStarter(ctrl, routine, infoClient, nil, nil).clientManager
			}

			starter.Backup(t.Context(), routine, now, model.BackupTypeFull)

			var state model.RoutineState
			runs.Describe(routine.Name, &state)
			assert.Equal(t, tt.wantJobs, state.Full != nil, "a run holds the started jobs")
			if tt.denied == nil {
				assert.Equal(t, 1, released, "the reservation is released once")
			}
		})
	}
}

// TestBackupStarter_WaitsForTheFirstListing checks that a start asks the start rules only once its
// cluster has been listed, so that they count the backups already running there.
func TestBackupStarter_WaitsForTheFirstListing(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		routine := testRoutine("daily")
		runs := NewRuns()
		var asked atomic.Bool
		startController := service.NewMockStartController(ctrl)
		startController.EXPECT().TryStart(routine, gomock.Any(), model.BackupTypeFull).DoAndReturn(
			func(*model.BackupRoutine, time.Time, model.BackupType) (func(), error) {
				asked.Store(true)
				return nil, fmt.Errorf("%w: full backup already in progress", service.ErrBackupSkipped)
			})
		reporter := service.NewMockBackupReporter(ctrl)
		reporter.EXPECT().Report(routine.Name, model.BackupTypeFull, gomock.Any(), gomock.Any(), gomock.Any())
		starter := &backupStarter{startController: startController, reporter: reporter, runs: runs}

		go starter.Backup(t.Context(), routine, time.Now(), model.BackupTypeFull)
		time.Sleep(time.Second)
		synctest.Wait()
		assert.False(t, asked.Load(), "asked before the cluster was listed")

		runs.update(routine.SourceCluster, nil, time.Now(), testRoutines{})
		synctest.Wait()
		assert.True(t, asked.Load())
	})
}

// newTestStarter returns a starter of routine's namespaces through infoClient, whose client is
// taken once and released once. With no namespaces, it resolves none.
func newTestStarter(
	ctrl *gomock.Controller, routine *model.BackupRoutine, infoClient *aerospike.MockInfoGetter, namespaces []string,
	runs *Runs,
) *backupStarter {
	client := aerospike.NewMockClient(ctrl)
	client.EXPECT().InfoClient().Return(infoClient).AnyTimes()
	clientManager := aerospike.NewMockClientManager(ctrl)
	clientManager.EXPECT().GetClient(gomock.Any(), routine.SourceCluster, gomock.Any(), gomock.Any()).
		Return(client, nil)
	clientManager.EXPECT().Close(client)
	namespaceResolver := aerospike.NewMockNamespaceResolver(ctrl)
	if namespaces != nil {
		namespaceResolver.EXPECT().ResolveNamespaces(gomock.Any(), routine, gomock.Any()).Return(namespaces, nil)
	}

	return &backupStarter{clientManager: clientManager, namespaceResolver: namespaceResolver, runs: runs}
}
