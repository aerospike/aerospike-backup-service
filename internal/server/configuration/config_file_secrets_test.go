package configuration

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Model storage types implement Stringer, so the "%+v" in storageManager.Write's
// error message does not expose cloud credentials.
func TestStorageManager_WriteErrorDoesNotLeakCredentials(t *testing.T) {
	var s model.Storage = &model.S3Storage{
		Bucket: "b",
		Auth:   &model.S3Authentication{KeyIDSecret: "AKIAEXAMPLE", AccessKeySecret: "super-secret-key"},
	}
	msg := fmt.Sprintf("failed to write configuration to storage %+v: %v", s, assert.AnError)
	assert.NotContains(t, msg, "super-secret-key")
	assert.NotContains(t, msg, "AKIAEXAMPLE")

	var az model.Storage = &model.AzureStorage{
		Endpoint: "https://x.blob.core.windows.net", ContainerName: "c",
		Auth: &model.AzureSharedKeyAuth{AccountName: "acct", AccountKey: "azure-account-key"},
	}
	assert.NotContains(t, fmt.Sprintf("%+v", az), "azure-account-key")

	var gcp model.Storage = &model.GcpStorage{BucketName: "b", KeyJSON: `{"private_key":"gcp-private"}`}
	assert.NotContains(t, fmt.Sprintf("%+v", gcp), "gcp-private")
}
