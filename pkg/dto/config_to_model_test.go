package dto

import (
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A configuration built from its DTO arrives with every routine pending, so the first apply
// schedules all of them. That is a consequence of installing the finished value into an
// empty Config, not of anything the build does per routine.
func TestConfig_ToModel_LeavesEveryRoutinePending(t *testing.T) {
	config, err := NewFromReader[Config](strings.NewReader(`service:
aerospike-clusters:
  cluster:
    seed-nodes:
      - host-name: 127.0.0.1
        port: 3000
storage:
  disk:
    local-storage:
      path: /backups
backup-routines:
  nightly:
    source-cluster: cluster
    storage: disk
    interval-cron: '@daily'
    namespaces: []
  hourly:
    source-cluster: cluster
    storage: disk
    interval-cron: '@hourly'
    namespaces: []
`), decoder.YAML)
	require.NoError(t, err)
	require.NoError(t, config.Validate())

	modelConfig, err := config.ToModel()
	require.NoError(t, err)

	assert.Equal(t, []string{"hourly", "nightly"}, modelConfig.PopInvalidatedRoutineNames())
}
