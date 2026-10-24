package archcheck_test

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/archcheck"
	"github.com/aerospike/aerospike-backup-service/v3/internal/doccheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allowed lists the newables that hold a component on purpose, each with the reason. The
// only honest reason is that a caller outside the newable's control fixes the method it is
// called through, so the component cannot come in as an argument. An entry that no longer
// matches a violation fails the test, so the list only shrinks.
//
//nolint:gochecknoglobals // a table, read by the tests in this package.
var allowed = map[string]string{
	"pkg/service.backupJob.orchestrator": "Quartz calls Execute(ctx) and nothing else; " +
		"the job is what binds the orchestrator to one routine.",
	"pkg/service/backupexecutor.closeOnWaitBackupHandler.clientManager": "the handler is a client lease " +
		"returned to callers that only know BackupHandler.Wait(ctx), which returns the client to the manager.",
}

// TestNewablesHoldNoComponents enforces the newable/injectable rule described in the
// package documentation: a type created at run time takes the components it needs as method
// arguments, it never keeps one in a field.
func TestNewablesHoldNoComponents(t *testing.T) {
	result, err := archcheck.Check(archcheck.Config{
		Dir:      doccheck.Root(t),
		Root:     "github.com/aerospike/aerospike-backup-service/v3/internal/app",
		Registry: "Components",
		Patterns: []string{"./pkg/...", "./internal/..."},
	})
	require.NoError(t, err)
	// An arch test that finds no components passes vacuously; this one is always wired.
	require.Contains(t, result.Components, "pkg/service/aerospike.ClientFactory",
		"the component set is missing a component the root wires; did the composition root move?")

	found := make(map[string]bool)
	for _, violation := range result.Violations {
		found[violation.Holder] = true
		_, ok := allowed[violation.Holder]
		assert.Truef(t, ok,
			"%s\nA newable is created at run time and takes components as method arguments. "+
				"If the method is fixed by a caller you do not control, add the field to allowed with the reason.",
			violation)
	}

	for holder := range allowed {
		assert.Truef(t, found[holder], "%s no longer holds a component; remove it from allowed", holder)
	}
}
