package decoder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeSecrets_ComplexFixture(t *testing.T) {
	original := testComplexConfig()
	redacted := RedactSecrets(original).(testConfig)

	// Simulate editing non-secret fields on a GET response payload.
	redacted.AerospikeClusters["cluster1"].Credentials.User = "updatedUser"
	redacted.StorageProviders["s3-main"] = testStorage{
		Name:            "updated-bucket",
		AccessKeyID:     Secret(redactedSecret),
		SecretAccessKey: Secret(redactedSecret),
	}

	MergeSecrets(&redacted, original)

	t.Run("restores literal secrets", func(t *testing.T) {
		assert.Equal(t, Secret(literalPassword), redacted.AerospikeClusters["cluster1"].Credentials.Password)
		assert.Equal(t, Secret(literalTLSPassword), redacted.AerospikeClusters["cluster1"].TLS.KeyfilePassword)
		assert.Equal(t, Secret(literalSecretKey), redacted.AerospikeClusters["cluster1"].Encryption.KeySecret)
		assert.Equal(t, Secret(literalAccessKey), redacted.StorageProviders["s3-main"].AccessKeyID)
		assert.Equal(t, Secret(literalSecretKey), redacted.StorageProviders["s3-main"].SecretAccessKey)
		assert.Equal(t, Secret(literalAccessKey), redacted.Routines[0].Storages[0].AccessKeyID)
		assert.Equal(t, Secret(literalPassword), redacted.Routines[0].Keys[0])
	})

	t.Run("preserves non-secret edits", func(t *testing.T) {
		assert.Equal(t, "updatedUser", redacted.AerospikeClusters["cluster1"].Credentials.User)
		assert.Equal(t, "updated-bucket", redacted.StorageProviders["s3-main"].Name)
	})

	t.Run("preserves valid secret refs", func(t *testing.T) {
		assert.Equal(t, Secret(validSecretRef), redacted.AerospikeClusters["cluster2"].Credentials.Password)
		assert.Equal(t, Secret(validSecretRef), redacted.StorageProviders["s3-ref"].AccessKeyID)
		assert.Equal(t, Secret(validSecretRef), redacted.Routines[0].Keys[1])
	})

	t.Run("preserves empty secrets", func(t *testing.T) {
		assert.Empty(t, redacted.AerospikeClusters["cluster3"].Credentials.Password)
		assert.Empty(t, redacted.StorageProviders["s3-ref"].SecretAccessKey)
	})

	t.Run("does not mutate original", func(t *testing.T) {
		assert.Equal(t, "testUser", original.AerospikeClusters["cluster1"].Credentials.User)
		assert.Equal(t, "main-bucket", original.StorageProviders["s3-main"].Name)
	})
}

func TestMergeSecrets_NewMapEntryWithSentinel(t *testing.T) {
	existing := testConfig{
		AerospikeClusters: map[string]*testCluster{},
	}

	incoming := testConfig{
		AerospikeClusters: map[string]*testCluster{
			"new-cluster": {
				Credentials: &testCredentials{
					User:     "newUser",
					Password: redactedSecret,
				},
			},
		},
	}

	MergeSecrets(&incoming, existing)

	assert.Equal(t, Secret(redactedSecret), incoming.AerospikeClusters["new-cluster"].Credentials.Password)
}

func TestMergeSecrets_ExplicitSecretUpdate(t *testing.T) {
	existing := testConfig{
		AerospikeClusters: map[string]*testCluster{
			"cluster1": {
				Credentials: &testCredentials{
					User:     "testUser",
					Password: literalPassword,
				},
			},
		},
	}

	incoming := testConfig{
		AerospikeClusters: map[string]*testCluster{
			"cluster1": {
				Credentials: &testCredentials{
					User:     "testUser",
					Password: "new-password",
				},
			},
		},
	}

	MergeSecrets(&incoming, existing)

	assert.Equal(t, Secret("new-password"), incoming.AerospikeClusters["cluster1"].Credentials.Password)
}

func TestMergeSecrets_ClearSecretWithEmptyString(t *testing.T) {
	existing := testConfig{
		AerospikeClusters: map[string]*testCluster{
			"cluster1": {
				Credentials: &testCredentials{
					User:     "testUser",
					Password: literalPassword,
				},
			},
		},
	}

	incoming := testConfig{
		AerospikeClusters: map[string]*testCluster{
			"cluster1": {
				Credentials: &testCredentials{
					User:     "testUser",
					Password: "",
				},
			},
		},
	}

	MergeSecrets(&incoming, existing)

	assert.Empty(t, incoming.AerospikeClusters["cluster1"].Credentials.Password)
}

func TestMergeSecrets_NilInput(t *testing.T) {
	original := testComplexConfig()

	require.NotPanics(t, func() {
		MergeSecrets(nil, original)
		MergeSecrets(&original, nil)
		MergeSecrets(nil, nil)
	})
}

func TestSecret_IsRedacted(t *testing.T) {
	assert.True(t, Secret(redactedSecret).IsRedacted())
	assert.False(t, Secret("real-password").IsRedacted())
	assert.False(t, Secret("").IsRedacted())
	assert.False(t, Secret(validSecretRef).IsRedacted())
}
