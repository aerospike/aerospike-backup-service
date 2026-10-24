package decoder

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
	"github.com/stretchr/testify/assert"
)

// Embedded types are exported, as in the DTOs: reflect cannot read an embedded unexported one.
type (
	InlineCredentials  testCredentials
	EmbeddedEncryption testEncryption
)

type testInlineHolder struct {
	InlineCredentials `yaml:",inline"`
	Label             string `yaml:"label"`
}

type testEmbeddedHolder struct {
	*EmbeddedEncryption
	Ignored redact.Secret `yaml:"-"`
}

func TestRedactedPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  []string
	}{
		{
			name: "placeholders in nested maps, slices and arrays",
			value: &testConfig{
				AerospikeClusters: map[string]*testCluster{
					"c1": {
						Credentials: &testCredentials{Password: redact.Placeholder},
						TLS:         &testTLS{KeyfilePassword: literalTLSPassword},
					},
					"c2": {Encryption: testEncryption{KeySecret: redact.Placeholder}},
				},
				StorageProviders: map[string]testStorage{
					"s1": {AccessKeyID: literalAccessKey, SecretAccessKey: redact.Placeholder},
				},
				Routines: []testBackupRoutine{
					{Keys: [2]redact.Secret{validSecretRef, redact.Placeholder}},
				},
			},
			want: []string{
				"aerospike-clusters.c1.credentials.password",
				"aerospike-clusters.c2.encryption.key-secret",
				"routines[0].keys[1]",
				"storage-providers.s1.secret-access-key",
			},
		},
		{
			name: "literal values and references are not placeholders",
			value: &testConfig{
				AerospikeClusters: map[string]*testCluster{
					"c1": {Credentials: &testCredentials{Password: literalPassword}},
				},
				Routines: []testBackupRoutine{{Keys: [2]redact.Secret{validSecretRef, ""}}},
			},
			want: nil,
		},
		{
			name:  "inline field is named by its parent",
			value: testInlineHolder{InlineCredentials: InlineCredentials{Password: redact.Placeholder}},
			want:  []string{"password"},
		},
		{
			name: "embedded pointer is inlined, skipped field is ignored",
			value: testEmbeddedHolder{
				EmbeddedEncryption: &EmbeddedEncryption{KeySecret: redact.Placeholder},
				Ignored:            redact.Placeholder,
			},
			want: []string{"key-secret"},
		},
		{
			name:  "a bare placeholder has an empty path",
			value: redact.Secret(redact.Placeholder),
			want:  []string{""},
		},
		{
			name:  "nil",
			value: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, RedactedPaths(tt.value))
		})
	}
}

// A value graph with a cycle is searched to the end.
func TestRedactedPaths_Cycle(t *testing.T) {
	t.Parallel()

	type node struct {
		Secret redact.Secret `yaml:"secret"`
		Next   *node         `yaml:"next"`
	}

	n := &node{Secret: redact.Placeholder}
	n.Next = n

	assert.Equal(t, []string{"secret"}, RedactedPaths(n))
}
