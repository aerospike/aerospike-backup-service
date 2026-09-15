package dto

import (
	"testing"

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
