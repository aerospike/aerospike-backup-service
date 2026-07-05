package handlers

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

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

	_ = svc.config.AddPolicy(entities.policyName, entities.policy)
	_ = svc.config.AddCluster(entities.clusterName, entities.cluster)
	_ = svc.config.AddStorage(entities.storageName, entities.storage)

	routine := &model.BackupRoutine{
		Name:          routineName,
		BackupPolicy:  entities.policy,
		SourceCluster: entities.cluster,
		Storage:       entities.storage,
		IntervalCron:  "@yearly",
		Namespaces:    []string{},
		Disabled:      disabled,
	}
	_ = svc.config.AddRoutine(routine)

	return entities
}
