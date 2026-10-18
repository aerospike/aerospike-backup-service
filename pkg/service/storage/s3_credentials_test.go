package storage

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Static credentials are resolved through the storage-level secret agent.
func TestS3WithCredentialsProvider_ResolvesThroughStorageAgent(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	agent := &model.SecretAgent{Address: "localhost"}
	resolver := secrets.NewMockResolver(ctrl)
	resolver.EXPECT().Resolve(gomock.Any(), agent, model.Secret("key-ref")).Return("key", nil)
	resolver.EXPECT().Resolve(gomock.Any(), agent, model.Secret("access-ref")).Return("access", nil)

	accessor := NewS3StorageAccessor(resolver)
	provider, err := accessor.withCredentialsProvider(
		t.Context(),
		&model.S3Authentication{KeyIDSecret: "key-ref", AccessKeySecret: "access-ref"},
		agent,
	)
	require.NoError(t, err)
	require.NotNil(t, provider)
}

// A storage that names a secret agent but no static credentials leaves the AWS default
// credential chain in place instead of installing empty static credentials.
func TestS3WithCredentialsProvider_NoStaticKeysKeepsDefaultChain(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	resolver := secrets.NewMockResolver(ctrl) // no EXPECT(): Resolve must not be called

	accessor := NewS3StorageAccessor(resolver)
	provider, err := accessor.withCredentialsProvider(
		t.Context(),
		nil,
		&model.SecretAgent{Address: "localhost"},
	)
	require.NoError(t, err)
	require.NotNil(t, provider)
}
