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

// A routine's name also goes into the info commands of server-side backup, so it is rejected in
// either backup mode when an info command cannot carry it.
func TestValidateRoutineName(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError string
	}{
		{name: "plain", value: "daily-backup"},
		{name: "semicolon", value: "a;b", expectError: `must not contain ";", "|", a line break or a NUL byte`},
		{name: "pipe", value: "daily|eu", expectError: `must not contain ";", "|", a line break or a NUL byte`},
		{name: "newline", value: "a\nb", expectError: `must not contain ";", "|", a line break or a NUL byte`},
		{name: "carriage return", value: "a\rb", expectError: `must not contain ";", "|", a line break or a NUL byte`},
		{name: "path segment rules apply", value: "a/b", expectError: "path separator"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRoutineName(tt.value)
			if tt.expectError == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.expectError)
		})
	}

	require.ErrorContains(t, validNameConfig("daily|eu", []NamespaceName{"ns"}).Validate(), "routine name",
		"Config.Validate checks routine names in scan mode too")
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

func TestValidateNames_SetOrBinName(t *testing.T) {
	tests := []struct {
		name      string
		names     []string
		maxLength int
		wantIs    error
		wantErr   string
	}{
		{name: "plain", names: []string{"set-1", "set_2$"}, maxLength: maxSetNameLength},
		{name: "longest set", names: []string{strings.Repeat("s", maxSetNameLength)}, maxLength: maxSetNameLength},
		{name: "longest bin", names: []string{strings.Repeat("b", maxBinNameLength)}, maxLength: maxBinNameLength},
		{name: "no names", maxLength: maxSetNameLength},
		{
			name:  "characters both backup modes carry",
			names: []string{"a.b", "a:b", "a=b", "a b", "a/b", "\u00e9", "\u65e5\u672c", "null"}, maxLength: maxSetNameLength,
		},
		{
			name: "set too long", names: []string{strings.Repeat("s", maxSetNameLength+1)}, maxLength: maxSetNameLength,
			wantIs: errInvalidValue, wantErr: `": must not exceed 63 bytes`,
		},
		{
			name: "bin too long in bytes", names: []string{strings.Repeat("\u00e9", 8)}, maxLength: maxBinNameLength,
			wantIs: errInvalidValue, wantErr: `must not exceed 15 bytes`,
		},
		{
			name: "comma", names: []string{"a,b"}, maxLength: maxSetNameLength,
			wantIs: errInvalidValue, wantErr: `list[0] "a,b": must not contain ",", ";", "|" or a control character`,
		},
		{name: "semicolon", names: []string{"a;b"}, maxLength: maxSetNameLength, wantIs: errInvalidValue, wantErr: "a;b"},
		{name: "pipe", names: []string{"a|b"}, maxLength: maxSetNameLength, wantIs: errInvalidValue, wantErr: "a|b"},
		{name: "LF", names: []string{"a\nb"}, maxLength: maxSetNameLength, wantIs: errInvalidValue, wantErr: "control"},
		{name: "tab", names: []string{"a\tb"}, maxLength: maxSetNameLength, wantIs: errInvalidValue, wantErr: "control"},
		{name: "NUL", names: []string{"a\x00b"}, maxLength: maxSetNameLength, wantIs: errInvalidValue, wantErr: "control"},
		{name: "DEL", names: []string{"a\x7fb"}, maxLength: maxSetNameLength, wantIs: errInvalidValue, wantErr: "control"},
		{
			name: "invalid UTF-8", names: []string{"a\xffb"}, maxLength: maxSetNameLength,
			wantIs: errInvalidValue, wantErr: "must be valid UTF-8",
		},
		{
			name: "empty", names: []string{"a", ""}, maxLength: maxSetNameLength,
			wantIs: errEmpty, wantErr: `"list[1]" required`,
		},
		{name: "duplicate", names: []string{"a", "a"}, maxLength: maxSetNameLength, wantErr: "duplicate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNames("list", tt.names, setOrBinName(tt.maxLength))
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			if tt.wantIs != nil {
				require.ErrorIs(t, err, tt.wantIs)
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// Set and bin names are checked in the routine and the restore policy, whatever the backup mode.
func TestValidate_SetAndBinNames(t *testing.T) {
	type validatable interface{ Validate() error }

	routine := func(sets, bins []string) validatable {
		return &BackupRoutine{
			SourceCluster: "cluster1", Storage: "storage1", IntervalCron: "0 0 * * * *",
			Namespaces: &[]NamespaceName{"ns1"}, SetList: sets, BinList: bins,
		}
	}
	policy := func(sets, bins []string) validatable {
		return &RestorePolicy{BaseRestorePolicy: BaseRestorePolicy{SetList: sets, BinList: bins}}
	}

	tests := []struct {
		name    string
		entity  validatable
		wantIs  error
		wantErr string
	}{
		{name: "routine", entity: routine([]string{"a.b"}, []string{"bin 1"})},
		{name: "restore policy", entity: policy([]string{"a.b"}, []string{"bin 1"})},
		{
			name: "routine set with a comma", entity: routine([]string{"a,b"}, nil),
			wantIs: errInvalidValue, wantErr: `set-list[0] "a,b"`,
		},
		{
			name: "restore policy bin with a semicolon", entity: policy(nil, []string{"a;b"}),
			wantIs: errInvalidValue, wantErr: `bin-list[0] "a;b"`,
		},
		{
			name: "routine set too long", entity: routine([]string{strings.Repeat("s", maxSetNameLength+1)}, nil),
			wantIs: errInvalidValue, wantErr: `set-list[0]`,
		},
		{
			name: "routine bin too long", entity: routine(nil, []string{"bin_name_of_16ch"}),
			wantIs: errInvalidValue, wantErr: `bin-list[0] "bin_name_of_16ch"`,
		},
		{
			name: "routine empty set", entity: routine([]string{""}, nil),
			wantIs: errEmpty, wantErr: `"set-list[0]" required`,
		},
		{
			name: "restore policy set with NUL", entity: policy([]string{"a\x00b"}, nil),
			wantIs: errInvalidValue, wantErr: `set-list[0] "a\x00b"`,
		},
		{
			name: "restore policy bin too long", entity: policy(nil, []string{"bin_name_of_16ch"}),
			wantIs: errInvalidValue, wantErr: `bin-list[0] "bin_name_of_16ch"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entity.Validate()
			if tt.wantIs == nil {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, tt.wantIs)
			require.ErrorContains(t, err, tt.wantErr)
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
