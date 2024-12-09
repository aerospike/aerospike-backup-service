package dto

import (
	"encoding/json"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	saClient "github.com/aerospike/backup-go/pkg/secret-agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigModelConversionIsLossless(t *testing.T) {
	// Step 1: Create a sample Config object with sample data
	secretAgentConfig := SecretAgentConfig{
		SecretAgentName: util.Ptr("agent1"),
	}

	originalConfig := &Config{
		ServiceConfig: NewBackupServiceConfigWithDefaultValues(),
		AerospikeClusters: map[string]*AerospikeCluster{
			"cluster1": {
				SeedNodes: []SeedNode{{HostName: "host", Port: 80}},
				Credentials: &Credentials{
					User:              util.Ptr("tester"),
					Password:          util.Ptr("psw"),
					SecretAgentConfig: secretAgentConfig,
				},
			},
			"cluster2": {
				SeedNodes: []SeedNode{{HostName: "host", Port: 80}},
				Credentials: &Credentials{
					User:     util.Ptr("tester"),
					Password: util.Ptr("psw"),
					SecretAgentConfig: SecretAgentConfig{
						SecretAgent: &SecretAgent{
							Address:        "host2",
							ConnectionType: saClient.ConnectionTypeTCP,
						},
					},
				},
			},
			"cluster3": {
				ClusterLabel: util.Ptr("No credentials"),
				SeedNodes:    []SeedNode{{HostName: "host", Port: 80}},
			},
		},
		Storage: map[string]*Storage{
			"aws 0": { // no secret agent
				S3Storage: &S3Storage{
					Bucket:          "bucket",
					S3Region:        "region",
					AccessKeyID:     util.Ptr("id"),
					SecretAccessKey: util.Ptr("key"),
				},
			},
			"aws 1": { // secret agent by name
				S3Storage: &S3Storage{
					Bucket:            "bucket",
					S3Region:          "region",
					AccessKeyID:       util.Ptr("id"),
					SecretAccessKey:   util.Ptr("key"),
					SecretAgentConfig: secretAgentConfig,
				},
			},
			"aws 2": { // secret agent by full definition
				S3Storage: &S3Storage{
					Bucket:          "bucket2",
					S3Region:        "region2",
					AccessKeyID:     util.Ptr("id2"),
					SecretAccessKey: util.Ptr("key2"),
					SecretAgentConfig: SecretAgentConfig{
						SecretAgent: &SecretAgent{
							Address:        "host3",
							ConnectionType: saClient.ConnectionTypeTCP,
						},
					},
				},
			},
			"gcp": {
				GcpStorage: &GcpStorage{
					KeyFile:           "key-file",
					BucketName:        "bucket",
					Path:              "path",
					Endpoint:          "http://localhost",
					SecretAgentConfig: secretAgentConfig,
				},
			},
			"gcp2": {
				GcpStorage: &GcpStorage{
					Key:        "key-json",
					BucketName: "bucket",
					Path:       "path",
					Endpoint:   "http://localhost",
					SecretAgentConfig: SecretAgentConfig{
						SecretAgent: &SecretAgent{
							Address:        "host3",
							ConnectionType: saClient.ConnectionTypeTCP,
						},
					},
				},
			},
			"azure 1": {
				AzureStorage: &AzureStorage{
					SecretAgentConfig: secretAgentConfig,
					Endpoint:          "http://localhost",
					ContainerName:     "container",
					Path:              "backup",
					AccountName:       "hello",
					AccountKey:        "world",
					TenantID:          "",
					ClientID:          "",
					ClientSecret:      "",
				},
			},
			"azure 2": {
				AzureStorage: &AzureStorage{
					SecretAgentConfig: SecretAgentConfig{
						SecretAgent: &SecretAgent{
							Address:        "host4",
							ConnectionType: saClient.ConnectionTypeUDS,
						},
					},
					Endpoint:      "http://localhost",
					ContainerName: "container",
					Path:          "backup",
					AccountName:   "",
					AccountKey:    "",
					TenantID:      "1",
					ClientID:      "2",
					ClientSecret:  "3",
				},
			},
		},
		BackupPolicies: map[string]*BackupPolicy{"policy1": {}},
		BackupRoutines: map[string]*BackupRoutine{"routine1": {
			BackupPolicy:  "policy1",
			SourceCluster: "cluster1",
			Storage:       "aws 1",
			IntervalCron:  "@daily",
		}},
		SecretAgents: map[string]*SecretAgent{"agent1": {
			Address:        "host",
			ConnectionType: saClient.ConnectionTypeTCP,
		}},
	}

	require.NoError(t, originalConfig.validate())
	indent, _ := json.MarshalIndent(originalConfig, "", "    ")
	t.Logf("\nOriginal config:\n%s\n", string(indent))

	// Step 2: Convert the Config to a model.Config
	nsValidator := &aerospike.NoopNamespaceValidator{}
	modelConfig, err := originalConfig.ToModel(nsValidator)
	require.NoError(t, err, "toModel should not return an error")

	// Step 3: Convert the model.Config back to a Config
	newConfig := NewConfigFromModel(modelConfig)

	// Step 4: Assert that the new Config matches the original Config
	assert.Equal(t, originalConfig, newConfig, "Config -> model.Config -> Config should be lossless")
}
