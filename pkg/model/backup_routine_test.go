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
				Timezone: time.FixedZone("test-zone", 3*60*60),
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
