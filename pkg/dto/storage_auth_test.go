package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A partial Azure auth block is a validation error instead of silently dropping the
// fields and falling back to anonymous access.
func TestAzureStorageValidate_RejectsPartialAuth(t *testing.T) {
	sharedKeyOnlyName := &AzureStorage{Endpoint: "https://x", ContainerName: "c", AccountName: "acct"}
	require.ErrorContains(t, sharedKeyOnlyName.Validate(), "requires both account-name and account-key")

	aadWithoutSecret := &AzureStorage{Endpoint: "https://x", ContainerName: "c", TenantID: "t", ClientID: "cid"}
	require.ErrorContains(t, aadWithoutSecret.Validate(), "requires tenant-id, client-id and client-secret")

	complete := &AzureStorage{Endpoint: "https://x", ContainerName: "c", AccountName: "acct", AccountKey: "key"}
	require.NoError(t, complete.Validate())
}

// An S3 storage configured with a secret agent but no static keys keeps the agent reference
// across the DTO -> model -> DTO round trip, so reading and rewriting the configuration
// does not drop it.
func TestS3Storage_KeepsSecretAgentWithoutStaticKeys(t *testing.T) {
	dtoCfg := &Config{
		SecretAgents: map[string]*SecretAgent{
			"agent": {ConnectionType: "tcp", Address: "localhost", Port: ptr.Of(Port(3005))},
		},
		Storage: map[string]*Storage{
			"s3": {S3Storage: &S3Storage{
				SecretAgentConfig: SecretAgentConfig{SecretAgentName: "agent"},
				Bucket:            "b", S3Region: "r",
			}},
		},
	}
	require.NoError(t, dtoCfg.Validate())

	m, err := dtoCfg.ToModel()
	require.NoError(t, err)
	s3Model := m.BackupConfigCopy().Storage["s3"].(*model.S3Storage)
	require.NotNil(t, s3Model.SecretAgent)
	assert.Equal(t, "localhost", s3Model.SecretAgent.Address)
	assert.Nil(t, s3Model.Auth, "no static credentials are configured")

	back := NewConfigFromModel(m)
	assert.Equal(t, "agent", back.Storage["s3"].S3Storage.SecretAgentName)
}

// Static credentials and the agent that resolves them survive the same round trip.
func TestS3Storage_KeepsSecretAgentWithStaticKeys(t *testing.T) {
	const keyRef, accessRef = "secrets:res:key", "secrets:res:access"

	dtoCfg := &Config{
		SecretAgents: map[string]*SecretAgent{
			"agent": {ConnectionType: "tcp", Address: "localhost", Port: ptr.Of(Port(3005))},
		},
		Storage: map[string]*Storage{
			"s3": {S3Storage: &S3Storage{
				SecretAgentConfig: SecretAgentConfig{SecretAgentName: "agent"},
				Bucket:            "b", S3Region: "r",
				AccessKeyID: keyRef, SecretAccessKey: accessRef,
			}},
		},
	}
	require.NoError(t, dtoCfg.Validate())

	m, err := dtoCfg.ToModel()
	require.NoError(t, err)
	s3Model := m.BackupConfigCopy().Storage["s3"].(*model.S3Storage)
	require.NotNil(t, s3Model.SecretAgent)
	require.NotNil(t, s3Model.Auth)
	assert.Equal(t, model.Secret(keyRef), s3Model.Auth.KeyIDSecret)
	assert.Equal(t, model.Secret(accessRef), s3Model.Auth.AccessKeySecret)

	back := NewConfigFromModel(m).Storage["s3"].S3Storage
	assert.Equal(t, "agent", back.SecretAgentName)
	assert.Equal(t, Secret(keyRef), back.AccessKeyID)
}
