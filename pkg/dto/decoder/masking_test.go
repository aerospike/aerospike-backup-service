package decoder

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactSecrets_ComplexFixture(t *testing.T) {
	original := testComplexConfig()
	literals := literalSecretsInConfig()
	nonSecrets := nonSecretValuesInConfig()

	t.Run("marshal json with redact", func(t *testing.T) {
		data, err := Marshal(&original, JSON, true)
		require.NoError(t, err)

		assertRedactedOutput(t, string(data), literals, nonSecrets)
	})

	t.Run("marshal yaml with redact", func(t *testing.T) {
		data, err := Marshal(&original, YAML, true)
		require.NoError(t, err)

		assertRedactedOutput(t, string(data), literals, nonSecrets)
	})

	t.Run("marshal json without redact preserves secrets", func(t *testing.T) {
		data, err := Marshal(&original, JSON, false)
		require.NoError(t, err)

		output := string(data)
		for _, literal := range literals {
			assert.Contains(t, output, literal)
		}
	})

	t.Run("redact secrets value", func(t *testing.T) {
		redacted := RedactSecrets(original).(testConfig)

		data, err := Marshal(&redacted, JSON, false)
		require.NoError(t, err)

		assertRedactedOutput(t, string(data), literals, nonSecrets)
	})

	t.Run("slog replace attr", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
			ReplaceAttr: RedactSecretsReplaceAttr(),
		}))

		logger.Info("config", slog.Any("config", original))

		assertRedactedOutput(t, buf.String(), literals, nonSecrets)
	})

	t.Run("does not mutate original", func(t *testing.T) {
		before := original
		_ = RedactSecrets(original)

		assert.Equal(t, before, original)
		assert.Equal(t, Secret(literalPassword), before.AerospikeClusters["cluster1"].Credentials.Password)
		assert.Equal(t, Secret(literalAccessKey), before.StorageProviders["s3-main"].AccessKeyID)
		assert.Equal(t, Secret(literalPassword), before.Routines[0].Keys[0])
	})

	t.Run("preserves time values", func(t *testing.T) {
		created := original.BackupHistory["routine1"][0].Created
		finished := original.BackupHistory["routine1"][0].Finished

		redacted := RedactSecrets(original).(testConfig)
		entry := redacted.BackupHistory["routine1"][0]

		assert.Equal(t, created, entry.Created)
		assert.Equal(t, finished, entry.Finished)
		assert.Equal(t, int64(1000), entry.Timestamp)
	})

	t.Run("preserves empty secret", func(t *testing.T) {
		redacted := RedactSecrets(original).(testConfig)

		assert.Empty(t, redacted.AerospikeClusters["cluster3"].Credentials.Password)
		assert.Empty(t, redacted.StorageProviders["s3-ref"].SecretAccessKey)

		clusterData, err := Marshal(redacted.AerospikeClusters["cluster3"], JSON, false)
		require.NoError(t, err)
		clusterOutput := string(clusterData)
		assert.Contains(t, clusterOutput, `"password":""`)
		assert.NotContains(t, clusterOutput, `"password":"`+redactedSecret+`"`)

		storageData, err := Marshal(redacted.StorageProviders["s3-ref"], JSON, false)
		require.NoError(t, err)
		storageOutput := string(storageData)
		assert.Contains(t, storageOutput, `"secret-access-key":""`)
		assert.NotContains(t, storageOutput, `"secret-access-key":"`+redactedSecret+`"`)
	})

	t.Run("preserves valid secret ref", func(t *testing.T) {
		redacted := RedactSecrets(original).(testConfig)

		assert.Equal(t, Secret(validSecretRef), redacted.AerospikeClusters["cluster2"].Credentials.Password)
		assert.Equal(t, Secret(validSecretRef), redacted.StorageProviders["s3-ref"].AccessKeyID)
		assert.Equal(t, Secret(validSecretRef), redacted.Routines[0].Keys[1])
	})

	t.Run("redacts malformed secret ref", func(t *testing.T) {
		redacted := RedactSecrets(original).(testConfig)

		assert.Equal(t, Secret(redactedSecret), redacted.AerospikeClusters["cluster2"].Encryption.KeySecret)

		data, err := Marshal(&redacted, JSON, false)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"key-secret":"`+redactedSecret+`"`)
		assert.NotContains(t, string(data), malformedSecretRef)
	})

	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, RedactSecrets(nil))
	})

	t.Run("nil pointer fields", func(t *testing.T) {
		redacted := RedactSecrets(original).(testConfig)

		assert.Nil(t, redacted.AerospikeClusters["cluster2"].TLS)
	})
}

func TestPasswordMasking(t *testing.T) {
	t.Run("without redaction", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		logger.Info("cluster credentials", slog.Any("credentials", testCreds))

		assert.Contains(t, buf.String(), literalPassword)
	})

	t.Run("with redaction", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
			ReplaceAttr: RedactSecretsReplaceAttr(),
		}))

		logger.Info("cluster credentials", slog.Any("credentials", testConfigWithCreds(testCreds)))

		output := buf.String()
		assert.Contains(t, output, `"user":"testUser"`)
		assert.NotContains(t, output, literalPassword)
	})
}

func assertRedactedOutput(t *testing.T, output string, literals, nonSecrets []string) {
	t.Helper()

	for _, literal := range literals {
		assert.NotContains(t, output, literal, "literal secret leaked")
	}

	for _, value := range nonSecrets {
		assert.Contains(t, output, value)
	}

	assert.Contains(t, output, redactedSecret)
}

func TestRedactSecrets_PreservesTime(t *testing.T) {
	created := time.UnixMilli(1000).UTC()
	finished := time.UnixMilli(5000).UTC()

	original := map[string][]testBackupDetails{
		"routine1": {
			{
				Key:       "backup1",
				Created:   created,
				Timestamp: 1000,
				Finished:  finished,
			},
		},
	}

	redacted := RedactSecrets(original).(map[string][]testBackupDetails)
	require.Len(t, redacted["routine1"], 1)
	assert.Equal(t, created, redacted["routine1"][0].Created)
	assert.Equal(t, finished, redacted["routine1"][0].Finished)
	assert.Equal(t, int64(1000), redacted["routine1"][0].Timestamp)
}

// credentialError mirrors aerospike.AerospikeError: exported fields on a type
// stored behind fmt.Errorf("%w"). slog.Any("error", err) walks that wrapError,
// whose err field is unexported, then Set panics when copying ResultCode.
type credentialError struct {
	ResultCode int
	message    string
}

func (e *credentialError) Error() string {
	return e.message
}

func TestRedactSecrets_WrappedError(t *testing.T) {
	err := fmt.Errorf("failed to connect to cluster: %w", &credentialError{
		ResultCode: 65,
		message:    "Invalid credential",
	})

	assert.Equal(t, err, RedactSecrets(err))
}

// RedactSecrets returns errors untouched, so a secret interpolated into an error
// message can only be masked by Secret's Stringer at construction time.
func TestSecret_MaskedInErrorMessages(t *testing.T) {
	secret := Secret(literalPassword)

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "%s verb",
			err:  fmt.Errorf("login failed for password %s", secret),
		},
		{
			name: "%v verb",
			err:  fmt.Errorf("login failed for password %v", secret),
		},
		{
			name: "wrapped with %w",
			err:  fmt.Errorf("connect cluster1: %w", fmt.Errorf("bad password %s", secret)),
		},
		{
			name: "joined errors",
			err:  errors.Join(fmt.Errorf("cluster1: %v", secret), fmt.Errorf("cluster2: %v", secret)),
		},
		{
			name: "secret nested in struct",
			err:  fmt.Errorf("invalid credentials %v", testCreds),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, tt.err.Error(), redactedSecret)
			assert.NotContains(t, tt.err.Error(), literalPassword)

			redacted, ok := RedactSecrets(tt.err).(error)
			require.True(t, ok)
			assert.NotContains(t, redacted.Error(), literalPassword)
		})
	}

	t.Run("secret ref preserved", func(t *testing.T) {
		err := fmt.Errorf("cannot resolve %v", Secret(validSecretRef))
		assert.Contains(t, err.Error(), validSecretRef)
	})

	t.Run("logged error attribute is masked", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
			ReplaceAttr: RedactSecretsReplaceAttr(),
		}))

		logger.Error("connect failed", slog.Any("error", fmt.Errorf("bad password %s", secret)))

		assert.Contains(t, buf.String(), redactedSecret)
		assert.NotContains(t, buf.String(), literalPassword)
	})

	t.Run("%q verb", func(t *testing.T) {
		err := fmt.Errorf("password %q", secret)
		assert.Contains(t, err.Error(), redactedSecret)
		assert.NotContains(t, err.Error(), literalPassword)
	})

	// Guards the one sharp edge: converting to string bypasses the Stringer, and
	// RedactSecrets cannot repair an error message after the fact.
	t.Run("raw string conversion leaks", func(t *testing.T) {
		err := fmt.Errorf("password %s", string(secret))
		assert.Contains(t, err.Error(), literalPassword)
		assert.Equal(t, err.Error(), RedactSecrets(err).(error).Error())
	})
}
