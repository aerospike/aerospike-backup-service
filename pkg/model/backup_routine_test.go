package model

import (
	"reflect"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/stretchr/testify/require"
)

// TestBackupRoutineCopy_GobRegistrations ensures that all interface implementations
// used by BackupRoutine are registered in gob and thus Copy() does not panic.
func TestBackupRoutineCopy_GobRegistrations(t *testing.T) {
	tests := []struct {
		name    string
		storage Storage
	}{
		{
			name:    "LocalStorage",
			storage: &LocalStorage{Path: "/tmp"},
		},
		{
			name:    "S3Storage",
			storage: &S3Storage{Bucket: "b", Path: "p"},
		},
		{
			name:    "GcpStorage",
			storage: &GcpStorage{BucketName: "b", Path: "p"},
		},
		{
			name: "AzureStorage with AzureSharedKeyAuth",
			storage: &AzureStorage{
				Path:          "p",
				Endpoint:      "e",
				ContainerName: "c",
				Auth:          &AzureSharedKeyAuth{AccountName: "a", AccountKey: "k"},
			},
		},
		{
			name: "AzureStorage with AzureADAuth",
			storage: &AzureStorage{
				Path:          "p",
				Endpoint:      "e",
				ContainerName: "c",
				Auth:          &AzureADAuth{TenantID: "t", ClientID: "i", ClientSecret: "s"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &BackupRoutine{
				Storage:  tt.storage,
				Timezone: Location{resolved: time.FixedZone("test-zone", 3*60*60), Configured: "test-zone"},
			}
			r.BackupPolicy = &BackupPolicy{}
			r.BackupPolicy.RetentionPolicy = &RetentionPolicy{}
			r.BackupPolicy.RetentionPolicy.FullBackups = optional.Of(1)

			// Assert Copy does not panic
			var out *BackupRoutine
			require.NotPanics(t, func() {
				out = r.Copy()
			}, "Copy() panicked; missing gob.Register for one of the interface implementations in Storage/AzureAuth")

			require.NotNil(t, out, "Copy() returned nil for non-nil receiver")
			require.NotSame(t, r, out, "Copy() returned the same pointer; expected a new instance")
			require.True(t,
				reflect.DeepEqual(r, out),
				"Copy() result is not deeply equal to source.\nsource: %#v\ncopy:   %#v", r, out)
		})
	}
}

func TestBackupRoutine_Schedules(t *testing.T) {
	zone := time.FixedZone("UTC+3", 3*60*60)
	routine := &BackupRoutine{
		IntervalCron:     "@daily",
		IncrIntervalCron: "@hourly",
		Timezone:         Location{resolved: zone},
	}

	require.Equal(t, Schedule{Cron: "@daily", Location: zone}, routine.FullSchedule())
	require.Equal(t, Schedule{Cron: "@hourly", Location: zone}, routine.IncrementalSchedule())
	require.True(t, routine.HasIncrementalSchedule())
}

func TestBackupRoutine_Schedules_DefaultTimezone(t *testing.T) {
	routine := &BackupRoutine{IntervalCron: "@daily"}

	require.Equal(t, DefaultScheduleTimezone, routine.FullSchedule().Location)
	require.False(t, routine.HasIncrementalSchedule())
}

func TestBackupRoutine_NextRun(t *testing.T) {
	routine := &BackupRoutine{
		IntervalCron:     "@daily",
		IncrIntervalCron: "@hourly",
	}

	next, err := routine.NextRun()

	require.NoError(t, err)
	require.NotNil(t, next.FullBackupTime())
	require.NotNil(t, next.IncrementalBackupTime())
}

func TestBackupRoutine_NextRun_WithoutIncremental(t *testing.T) {
	routine := &BackupRoutine{IntervalCron: "@daily"}

	next, err := routine.NextRun()

	require.NoError(t, err)
	require.NotNil(t, next.FullBackupTime())
	require.Nil(t, next.IncrementalBackupTime())
}

func TestBackupRoutine_NextRun_InvalidCron(t *testing.T) {
	tests := map[string]struct {
		routine *BackupRoutine
		wantErr string
	}{
		"invalid full cron": {
			routine: &BackupRoutine{IntervalCron: "not a cron"},
			wantErr: "failed to parse full backup cron",
		},
		"invalid incremental cron": {
			routine: &BackupRoutine{IntervalCron: "@daily", IncrIntervalCron: "not a cron"},
			wantErr: "failed to parse incremental backup cron",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := tt.routine.NextRun()

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestBackupRoutine_BacksUpWholeCluster(t *testing.T) {
	tests := map[string]struct {
		namespaces []string
		want       bool
	}{
		"no namespaces configured backs up the whole cluster": {namespaces: nil, want: true},
		"explicit empty list backs up the whole cluster":      {namespaces: []string{}, want: true},
		"configured namespaces back up only those":            {namespaces: []string{"ns1"}, want: false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			routine := &BackupRoutine{Namespaces: tt.namespaces}

			require.Equal(t, tt.want, routine.BacksUpWholeCluster())
		})
	}
}
