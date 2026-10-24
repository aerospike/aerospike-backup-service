package dto

import (
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/stretchr/testify/require"
)

const keySecretBase = `
secret-agents:
  sa: {connection-type: tcp, address: secret-agent.aerospike.svc, port: 3005}
aerospike-clusters:
  c1: {seed-nodes: [{host-name: localhost, port: 3000}]}
storage:
  s1: {local-storage: {path: /tmp/backups}}
backup-policies:
  enc-policy:
    encryption: {mode: AES256, key-secret: "secrets:abs:enckey"}
  literal-policy:
    encryption: {mode: AES256, key-secret: "literal-key"}
`

func keySecretRoutine(name, policy, agent string) string {
	routine := `
  ` + name + `:
    backup-policy: ` + policy + `
    source-cluster: c1
    storage: s1
    interval-cron: "@daily"
    namespaces: [test]
`
	if agent != "" {
		routine += "    secret-agent: " + agent + "\n"
	}

	return routine
}

// An encryption key-secret is resolved with the agent of the routine that runs the policy, so
// it is each routine using a Secret Agent reference that must name an agent - not the policy,
// which may be defined before any routine uses it.
func TestConfig_Validate_KeySecretAgent(t *testing.T) {
	tests := []struct {
		name     string
		routines string
		wantErr  string
	}{
		{
			name:     "policy with a reference and no routine yet",
			routines: "",
		},
		{
			name:     "routine with an agent",
			routines: keySecretRoutine("r1", "enc-policy", "sa"),
		},
		{
			name:     "routine without an agent on a literal key",
			routines: keySecretRoutine("r1", "literal-policy", ""),
		},
		{
			name:     "routine without an agent on a reference",
			routines: keySecretRoutine("r1", "enc-policy", ""),
			wantErr:  "backup routine 'r1' validation error: secret-agent",
		},
		{
			name: "policy shared by a routine with an agent and one without",
			routines: keySecretRoutine("r1", "enc-policy", "sa") +
				keySecretRoutine("r2", "enc-policy", ""),
			wantErr: "backup routine 'r2' validation error: secret-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := keySecretBase
			if tt.routines != "" {
				yaml += "backup-routines:" + tt.routines
			}
			config, err := NewFromReader[Config](strings.NewReader(yaml), decoder.YAML)
			require.NoError(t, err)

			err = config.Validate()

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, errValidation)
			require.ErrorContains(t, err, tt.wantErr)
			require.ErrorContains(t, err, `policy 'enc-policy' reads the encryption key-secret "secrets:abs:enckey"`)
		})
	}
}

// A malformed reference is still rejected on the policy itself, used or not.
func TestConfig_Validate_MalformedKeySecretOnUnusedPolicy(t *testing.T) {
	config, err := NewFromReader[Config](strings.NewReader(`
backup-policies:
  enc-policy:
    encryption: {mode: AES256, key-secret: "secrets:abs"}
`), decoder.YAML)
	require.NoError(t, err)

	require.ErrorContains(t, config.Validate(), "must be in the form secrets:<resource>:<key>")
}
