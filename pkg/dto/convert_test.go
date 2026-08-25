package dto

import (
	"encoding/json"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	saClient "github.com/aerospike/backup-go/pkg/secret-agent"
	"github.com/stretchr/testify/require"
)

var secretAgentConfig = SecretAgentConfig{
	SecretAgentName: "agent1",
}

var originalConfig = &Config{
	ServiceConfig: ServiceConfig{
		HTTPServer: &HTTPServerConfig{
			ListenerConfig: ListenerConfig{Address: "localhost"},
		},
		Logger: &LoggerConfig{
			Level: "INFO",
			FileWriter: &FileLoggerConfig{
				Filename: "log",
			},
		},
	},
	AerospikeClusters: map[string]*AerospikeCluster{
		"cluster1": {
			SeedNodes: []SeedNode{{HostName: "host", Port: 80}},
			Credentials: &Credentials{
				User:              "tester",
				Password:          "psw",
				SecretAgentConfig: secretAgentConfig,
			},
		},
		"cluster2": {
			SeedNodes: []SeedNode{{HostName: "host", Port: 80}},
			Credentials: &Credentials{
				User:     "tester",
				Password: "psw",
				SecretAgentConfig: SecretAgentConfig{
					SecretAgent: &SecretAgent{
						Address:        "host2",
						ConnectionType: saClient.ConnectionTypeTCP,
					},
				},
			},
		},
		"cluster3": {
			ClusterLabel: "No credentials",
			SeedNodes:    []SeedNode{{HostName: "host", Port: 80}},
		},
	},
	Storage: map[string]*Storage{
		"aws 0": { // no secret agent
			S3Storage: &S3Storage{
				Bucket:          "bucket",
				S3Region:        "region",
				AccessKeyID:     "id",
				SecretAccessKey: "key",
			},
		},
		"aws 1": { // secret agent by name
			S3Storage: &S3Storage{
				Bucket:            "bucket",
				S3Region:          "region",
				AccessKeyID:       "id",
				SecretAccessKey:   "key",
				SecretAgentConfig: secretAgentConfig,
			},
		},
		"aws 2": { // secret agent by full definition
			S3Storage: &S3Storage{
				Bucket:          "bucket2",
				S3Region:        "region2",
				AccessKeyID:     "id2",
				SecretAccessKey: "key2",
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
		Namespaces:    ptr.Of([]string{}),
	}},
	SecretAgents: map[string]*SecretAgent{"agent1": {
		Address:        "host",
		ConnectionType: saClient.ConnectionTypeTCP,
	}},
}

func TestConfigModelConversionIsLossless(t *testing.T) {
	configJSON, _ := json.MarshalIndent(originalConfig, "", "    ")
	t.Logf("\nOriginal config:\n%s\n", string(configJSON))

	require.NoError(t, originalConfig.Validate(ValidationDefault))

	// Convert the Config to a model.Config
	modelConfig, err := originalConfig.ToModel()
	require.NoError(t, err, "toModel should not return an error")

	// Convert the model.Config back to a Config
	newConfig := NewConfigFromModel(modelConfig)

	// Assert that the new Config matches the original Config
	require.Equal(t, originalConfig, newConfig, "Config -> model.Config -> Config should be lossless")
}

func TestConfigValidation(t *testing.T) {
	configJSON, _ := json.Marshal(originalConfig)
	require.NoError(t, originalConfig.Validate(ValidationDefault))
	configJSONAfter, _ := json.Marshal(originalConfig)
	require.JSONEq(t, string(configJSON), string(configJSONAfter), "validation should not change the config")
}
