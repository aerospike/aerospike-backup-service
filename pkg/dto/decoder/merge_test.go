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
		AccessKeyID:     redactedSecret,
		SecretAccessKey: redactedSecret,
	}

	err := MergeSecrets(&redacted, original)
	require.NoError(t, err)

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

func TestMergeSecrets_NewMapEntryWithSentinel_ReturnsError(t *testing.T) {
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

	err := MergeSecrets(&incoming, existing)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use redacted secret")
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

	err := MergeSecrets(&incoming, existing)
	require.NoError(t, err)

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

	err := MergeSecrets(&incoming, existing)
	require.NoError(t, err)

	assert.Empty(t, incoming.AerospikeClusters["cluster1"].Credentials.Password)
}

func TestMergeSecrets_NilInput(t *testing.T) {
	original := testComplexConfig()

	require.NotPanics(t, func() {
		err1 := MergeSecrets(nil, original)
		assert.NoError(t, err1)
		err2 := MergeSecrets(&original, nil)
		assert.NoError(t, err2)
		err3 := MergeSecrets(nil, nil)
		assert.NoError(t, err3)
	})
}

func TestMergeSecrets_NewEntityWithSentinel_StorageSecret(t *testing.T) {
	existing := testConfig{
		StorageProviders: map[string]testStorage{},
	}

	incoming := testConfig{
		StorageProviders: map[string]testStorage{
			"new-storage": {
				Name:            "new-bucket",
				AccessKeyID:     redactedSecret,
				SecretAccessKey: redactedSecret,
			},
		},
	}

	err := MergeSecrets(&incoming, existing)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use redacted secret")
}

func TestMergeSecrets_ExistingEntityWithSentinel_RestoredCorrectly(t *testing.T) {
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
					Password: redactedSecret,
				},
			},
		},
	}

	err := MergeSecrets(&incoming, existing)
	require.NoError(t, err)
	assert.Equal(t, Secret(literalPassword), incoming.AerospikeClusters["cluster1"].Credentials.Password)
}

func TestSecret_IsRedacted(t *testing.T) {
	assert.True(t, Secret(redactedSecret).IsRedacted())
	assert.False(t, Secret("real-password").IsRedacted())
	assert.False(t, Secret("").IsRedacted())
	assert.False(t, Secret(validSecretRef).IsRedacted())
}
