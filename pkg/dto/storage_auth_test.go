package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An S3 storage configured with a secret agent but no static keys must keep the agent reference
// across the DTO -> model -> DTO round trip.
func TestS3Storage_KeepsSecretAgentWithoutStaticKeys(t *testing.T) {
	t.Skip("BKRS-419: toModel drops the secret-agent reference when no static keys are set")

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

	back := NewConfigFromModel(m)
	assert.Equal(t, "agent", back.Storage["s3"].S3Storage.SecretAgentName)
}

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
