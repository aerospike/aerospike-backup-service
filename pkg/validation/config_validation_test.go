package validation

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
)

func TestValidateConfiguration(t *testing.T) {
	config := &dto.Config{
		AerospikeClusters: map[string]*dto.AerospikeCluster{
			"cluster1": {
				SeedNodes: []dto.SeedNode{
					{
						HostName: "localhost",
						Port:     3000,
						TLSName:  "tls-name",
					},
				},
				TLS: &dto.TLS{ // validation will pass even with non-existent files
					ClientTLS: dto.ClientTLS{
						Name:     ptr.Of("name"),
						CAFile:   ptr.Of("/tmp/non-existent-ca-file"),
						Keyfile:  ptr.Of("/tmp/non-existent-key-file"),
						Certfile: ptr.Of("/tmp/non-existent-cert-file"),
					},
				},
			},
		},
	}

	err := ValidateConfiguration(config)
	assert.NoError(t, err)
}

func TestValidateRestoreRequest(t *testing.T) {
	config := &dto.Config{}
	request := &dto.RestoreRequest{
		BackupDataPath: "/path",
		DestinationClusterConfig: dto.DestinationClusterConfig{
			Cluster: &dto.AerospikeCluster{
				SeedNodes: []dto.SeedNode{
					{
						HostName: "localhost",
						Port:     3000,
						TLSName:  "tls-name",
					},
				},
				TLS: &dto.TLS{ // validation will pass even with non-existent files
					ClientTLS: dto.ClientTLS{
						Name:     ptr.Of("name"),
						CAFile:   ptr.Of("/tmp/non-existent-ca-file"),
						Keyfile:  ptr.Of("/tmp/non-existent-key-file"),
						Certfile: ptr.Of("/tmp/non-existent-cert-file"),
					},
				},
			},
		},
		StorageConfig: dto.StorageConfig{
			Name: "storage1",
		},
	}

	// need to add a dummy storage to the config
	config.Storage = map[string]*dto.Storage{
		"storage1": {
			S3Storage: &dto.S3Storage{
				Bucket:   "bucket",
				S3Region: "us-east-1",
			},
		},
	}

	err := ValidateRestoreRequest(request, config)
	assert.NoError(t, err)
}

func TestValidateRestoreTimestampRequest(t *testing.T) {
	config := &dto.Config{}
	request := &dto.RestoreTimestampRequest{
		Time:    time.Now().UnixMilli(),
		Routine: "routine1",
		DestinationClusterConfig: dto.DestinationClusterConfig{
			Name: "cluster1",
		},
	}

	// need to add a dummy routine to the config
	config.BackupRoutines = map[string]*dto.BackupRoutine{
		"routine1": {
			Storage:       "storage1",
			SourceCluster: "cluster1",
			IntervalCron:  "@daily",
			Namespaces:    ptr.Of([]string{}),
		},
	}

	config.Storage = map[string]*dto.Storage{
		"storage1": {
			S3Storage: &dto.S3Storage{
				Bucket:   "bucket",
				S3Region: "us-east-1",
			},
		},
	}

	config.AerospikeClusters = map[string]*dto.AerospikeCluster{
		"cluster1": {
			SeedNodes: []dto.SeedNode{
				{
					HostName: "localhost",
					Port:     3000,
					TLSName:  "tls-name",
				},
			},
			TLS: &dto.TLS{ // validation will pass even with non-existent files
				ClientTLS: dto.ClientTLS{
					Name:     ptr.Of("name"),
					CAFile:   ptr.Of("/tmp/non-existent-ca-file"),
					Keyfile:  ptr.Of("/tmp/non-existent-key-file"),
					Certfile: ptr.Of("/tmp/non-existent-cert-file"),
				},
			},
		},
	}
	err := ValidateRestoreTimestampRequest(request, config)
	assert.NoError(t, err)
}
