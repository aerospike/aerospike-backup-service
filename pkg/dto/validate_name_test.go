package dto

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validNameConfig(routineName string, namespaces []NamespaceName) *Config {
	return &Config{
		AerospikeClusters: map[string]*AerospikeCluster{
			"c": {SeedNodes: []SeedNode{{HostName: "localhost", Port: 3000}}},
		},
		Storage: map[string]*Storage{
			"s": {LocalStorage: &LocalStorage{Path: "/var/backups"}},
		},
		BackupRoutines: map[string]*BackupRoutine{
			routineName: {
				SourceCluster: "c",
				Storage:       "s",
				IntervalCron:  "@daily",
				Namespaces:    &namespaces,
			},
		},
	}
}

// Entity names (map keys) and namespace names end up in storage paths through PathService, so
// validation accepts only a single path segment: no separators, no "..".
func TestConfigValidate_RejectsPathSegmentsInNames(t *testing.T) {
	require.Error(t, validNameConfig("../../escaped-routine", []NamespaceName{"ns"}).Validate(),
		"routine name with path traversal")
	require.Error(t, validNameConfig("a/b", []NamespaceName{"ns"}).Validate(),
		"routine name with a path separator")
	require.Error(t, validNameConfig("r", []NamespaceName{"../../escaped-ns"}).Validate(),
		"namespace with path traversal")
	require.NoError(t, validNameConfig("daily-backup", []NamespaceName{"ns"}).Validate())
}

func TestValidateEntityName(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError string
	}{
		{name: "plain", value: "daily-backup"},
		{name: "dotted", value: "cluster.prod"},
		{name: "spaced", value: "my routine"},
		{name: "leading dot", value: ".hidden"},
		{name: "empty", value: "", expectError: "required"},
		{name: "single space", value: " ", expectError: "must not be blank"},
		{name: "only spaces", value: "   ", expectError: "must not be blank"},
		{name: "only a tab", value: "\t", expectError: "must not be blank"},
		{name: "leading space", value: " daily", expectError: "must not start or end with whitespace"},
		{name: "trailing space", value: "daily ", expectError: "must not start or end with whitespace"},
		{name: "trailing newline", value: "daily\n", expectError: "must not start or end with whitespace"},
		{name: "non-breaking space", value: "daily\u00a0", expectError: "must not start or end with whitespace"},
		{name: "traversal", value: "..", expectError: "path traversal"},
		{name: "current directory", value: ".", expectError: "path traversal"},
		{name: "relative traversal", value: "../../escape", expectError: "path separator"},
		{name: "separator", value: "a/b", expectError: "path separator"},
		{name: "windows separator", value: `a\b`, expectError: "path separator"},
		{name: "home expansion", value: "~/backups", expectError: "path separator"},
		{name: "home", value: "~", expectError: `must not start with "~"`},
		{name: "nul byte", value: "a\x00b", expectError: "NUL byte"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEntityName("routine name", tt.value)
			if tt.expectError == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestNamespaceName_Validate(t *testing.T) {
	tests := []struct {
		name        string
		value       NamespaceName
		expectError string
	}{
		{name: "plain", value: "source-ns1"},
		{name: "underscore and dollar", value: "ns_1$"},
		{name: "longest allowed", value: NamespaceName(strings.Repeat("n", maxNamespaceNameLength))},
		{name: "empty", value: "", expectError: "required"},
		{
			name:        "too long",
			value:       NamespaceName(strings.Repeat("n", maxNamespaceNameLength+1)),
			expectError: "must not exceed 31 bytes",
		},
		{name: "reserved", value: "null", expectError: "reserved"},
		{name: "traversal", value: "../../escaped-ns", expectError: "Latin letters"},
		{name: "separator", value: "a/b", expectError: "Latin letters"},
		{name: "dot", value: "a.b", expectError: "Latin letters"},
		{name: "nul byte", value: "a\x00b", expectError: "Latin letters"},
		{name: "non-latin", value: "имя", expectError: "Latin letters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errValidationInvalidName("namespaces[0]", string(tt.value), tt.value.Validate())
			if tt.expectError == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

// Every entity name is checked, not just the routine's.
func TestConfigValidate_RejectsPathSegmentsInEveryEntityName(t *testing.T) {
	withStorage := validNameConfig("r", []NamespaceName{"ns"})
	withStorage.Storage = map[string]*Storage{
		"../escaped": {LocalStorage: &LocalStorage{Path: "/var/backups"}},
	}
	withStorage.BackupRoutines["r"].Storage = "../escaped"
	require.ErrorContains(t, withStorage.Validate(), "storage name")

	withCluster := validNameConfig("r", []NamespaceName{"ns"})
	withCluster.AerospikeClusters = map[string]*AerospikeCluster{
		"../escaped": {SeedNodes: []SeedNode{{HostName: "localhost", Port: 3000}}},
	}
	withCluster.BackupRoutines["r"].SourceCluster = "../escaped"
	require.ErrorContains(t, withCluster.Validate(), "cluster name")

	withPolicy := validNameConfig("r", []NamespaceName{"ns"})
	withPolicy.BackupPolicies = map[string]*BackupPolicy{"../escaped": {}}
	require.ErrorContains(t, withPolicy.Validate(), "policy name")

	withAgent := validNameConfig("r", []NamespaceName{"ns"})
	withAgent.SecretAgents = map[string]*SecretAgent{
		"../escaped": {ConnectionType: "tcp", Address: "localhost"},
	}
	require.ErrorContains(t, withAgent.Validate(), "secret agent name")
}

// A restore remaps one namespace onto another, and both names are namespace names: an invalid
// one is rejected here rather than reaching the cluster.
func TestRestoreNamespace_Validate(t *testing.T) {
	tests := []struct {
		name        string
		namespace   RestoreNamespace
		expectError string
	}{
		{name: "valid", namespace: RestoreNamespace{Source: "source-ns", Destination: "dest-ns"}},
		{
			name:        "missing source",
			namespace:   RestoreNamespace{Destination: "dest-ns"},
			expectError: `"source" required`,
		},
		{
			name:        "missing destination",
			namespace:   RestoreNamespace{Source: "source-ns"},
			expectError: `"destination" required`,
		},
		{
			name:        "invalid source",
			namespace:   RestoreNamespace{Source: "../../escaped", Destination: "dest-ns"},
			expectError: "source",
		},
		{
			name:        "invalid destination",
			namespace:   RestoreNamespace{Source: "source-ns", Destination: "null"},
			expectError: "reserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.namespace.Validate()
			if tt.expectError == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}
