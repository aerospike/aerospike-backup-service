package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	saClient "github.com/aerospike/backup-go/pkg/secret-agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigModelConversionIsLossless(t *testing.T) {
	// Step 1: Create a sample Config object with sample data
	originalConfig := &Config{
		ServiceConfig: NewBackupServiceConfigWithDefaultValues(),
		AerospikeClusters: map[string]*AerospikeCluster{
			"cluster1": {
				SeedNodes: []SeedNode{{HostName: "host", Port: 80}},
				Credentials: &Credentials{
					User:              util.Ptr("tester"),
					PasswordKeySecret: util.Ptr("psw"),
					SecretAgentName:   util.Ptr("agent1"),
				},
			},
			"cluster2": {
				SeedNodes: []SeedNode{{HostName: "host", Port: 80}},
				Credentials: &Credentials{
					User:              util.Ptr("tester"),
					PasswordKeySecret: util.Ptr("psw"),
					SecretAgent: &SecretAgent{
						Address:        "host2",
						ConnectionType: saClient.ConnectionTypeTCP,
					},
				},
			}},
		Storage: map[string]*Storage{
			"storage1": {
				S3Storage: &S3Storage{
					Bucket:          "bucket",
					S3Region:        "region",
					AccessKeyID:     util.Ptr("id"),
					SecretAccessKey: util.Ptr("key"),
					SecretAgentName: util.Ptr("agent1"),
				},
			},
			"storage2": {
				S3Storage: &S3Storage{
					Bucket:          "bucket2",
					S3Region:        "region2",
					AccessKeyID:     util.Ptr("id2"),
					SecretAccessKey: util.Ptr("key2"),
					SecretAgent: &SecretAgent{
						Address:        "host3",
						ConnectionType: saClient.ConnectionTypeTCP,
					},
				},
			},
		},
		BackupPolicies: map[string]*BackupPolicy{"policy1": {}},
		BackupRoutines: map[string]*BackupRoutine{"routine1": {
			BackupPolicy:  "policy1",
			SourceCluster: "cluster1",
			Storage:       "storage1",
			IntervalCron:  "@daily",
		}},
		SecretAgents: map[string]*SecretAgent{"agent1": {
			Address:        "host",
			ConnectionType: saClient.ConnectionTypeTCP,
		}},
	}

	nsValidator := &service.NoopNamespaceValidator{}
	modelConfig, err := originalConfig.ToModel(nsValidator)
	require.NoError(t, err, "ToModel should not return an error")

	// Step 3: Convert the model.Config back to a Config
	newConfig := NewConfigFromModel(modelConfig)

	// Step 4: Assert that the new Config matches the original Config
	assert.Equal(t, originalConfig, newConfig, "Config -> model.Config -> Config should be lossless")
}
