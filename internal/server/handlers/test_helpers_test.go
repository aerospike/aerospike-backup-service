package handlers

import (
	"testing"

	servertls "github.com/aerospike/aerospike-backup-service/v3/internal/server/tlsconfig"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"go.uber.org/mock/gomock"
)

func newMockTLSProber(ctrl *gomock.Controller) *servertls.MockProber {
	prober := servertls.NewMockProber(ctrl)
	prober.EXPECT().Probe(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	return prober
}

type validTestBackupEntities struct {
	policyName  string
	clusterName string
	storageName string
	routineName string
	policy      *model.BackupPolicy
	cluster     *model.AerospikeCluster
	storage     model.Storage
}

func addValidBackupConfig(svc *Service) validTestBackupEntities {
	return addValidBackupRoutine(svc, "routine1", false)
}

func addValidBackupRoutine(svc *Service, routineName string, disabled bool) validTestBackupEntities {
	entities := validTestBackupEntities{
		policyName:  "test-policy",
		clusterName: "cluster1",
		storageName: "storage1",
		routineName: routineName,
		policy:      &model.BackupPolicy{Parallel: ptr.Of(8)},
		cluster: &model.AerospikeCluster{
			SeedNodes: []model.SeedNode{{HostName: "localhost", Port: 3000}},
		},
		storage: &model.LocalStorage{Path: "/tmp/backup"},
	}

	backupConfig := svc.config.BackupConfigCopy()
	_ = backupConfig.AddPolicy(entities.policyName, entities.policy)
	_ = backupConfig.AddCluster(entities.clusterName, entities.cluster)
	_ = backupConfig.AddStorage(entities.storageName, entities.storage)

	routine := &model.BackupRoutine{
		Name:          routineName,
		BackupPolicy:  entities.policy,
		SourceCluster: entities.cluster,
		Storage:       entities.storage,
		IntervalCron:  "@yearly",
		Namespaces:    []string{},
		Disabled:      disabled,
	}
	_ = backupConfig.AddRoutine(routine)
	svc.config.SetBackupConfig(backupConfig)

	return entities
}

// installInto applies edit to a copy of cfg's backup configuration and installs the result,
// the way every production change reaches the holder.
func installInto(cfg *model.Config, edit func(*model.BackupConfig) error) error {
	backupConfig := cfg.BackupConfigCopy()
	if err := edit(backupConfig); err != nil {
		return err
	}
	cfg.SetBackupConfig(backupConfig)

	return nil
}

func addCluster(cfg *model.Config, name string, cluster *model.AerospikeCluster) error {
	return installInto(cfg, func(bc *model.BackupConfig) error { return bc.AddCluster(name, cluster) })
}

func addStorage(cfg *model.Config, name string, storage model.Storage) error {
	return installInto(cfg, func(bc *model.BackupConfig) error { return bc.AddStorage(name, storage) })
}

func addPolicy(cfg *model.Config, name string, policy *model.BackupPolicy) error {
	return installInto(cfg, func(bc *model.BackupConfig) error { return bc.AddPolicy(name, policy) })
}

func addRoutine(cfg *model.Config, routine *model.BackupRoutine) error {
	return installInto(cfg, func(bc *model.BackupConfig) error { return bc.AddRoutine(routine) })
}

// newServiceWithNamespaceValidator is setupTestService plus a permissive namespace validator,
// which the config-changing handlers call under the lock.
func newServiceWithNamespaceValidator(t *testing.T) *Service {
	t.Helper()

	svc := setupTestService(t)
	nsValidator := aerospike.NewMockNamespaceValidator(gomock.NewController(t))
	nsValidator.EXPECT().Validate(gomock.Any(), gomock.Any()).AnyTimes()
	svc.nsValidator = nsValidator

	return svc
}
