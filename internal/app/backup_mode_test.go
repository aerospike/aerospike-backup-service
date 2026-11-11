package app

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/stretchr/testify/assert"
)

// TestNewComponentsForMode checks that the configured backup mode decides which package the
// mode's components come from, and that no component of a mode is left unset.
func TestNewComponentsForMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mode        model.BackupMode
		wantPackage string
	}{
		{name: "scan by default", wantPackage: "scanbackup"},
		{name: "scan", mode: model.BackupModeScan, wantPackage: "scanbackup"},
		{name: "server", mode: model.BackupModeServer, wantPackage: "serverbackup"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := model.NewConfig()
			config.ServiceConfig.Backup = &model.BackupCommonConfig{BackupMode: tt.mode}

			resolver := secrets.NewResolver()
			s3Accessor := storage.NewS3StorageAccessor(resolver)
			clientManager, _ := newAerospikeLayer(resolver)

			mode := newComponentsForMode(
				config, newStorageOperations(resolver, s3Accessor), s3Accessor, clientManager,
				service.NewRestoreJobsHolder())

			fields := reflect.ValueOf(mode)
			for i := range fields.NumField() {
				assert.Falsef(t, fields.Field(i).IsNil(), "%s is not set", fields.Type().Field(i).Name)
			}

			wantPrefix := "*" + tt.wantPackage + "."
			assert.Contains(t, fmt.Sprintf("%T", mode.backupReader), wantPrefix)
			assert.Contains(t, fmt.Sprintf("%T", mode.routineRunner), wantPrefix)
			assert.Contains(t, fmt.Sprintf("%T", mode.restoreManager), wantPrefix)
		})
	}
}
