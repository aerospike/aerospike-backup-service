package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func TestDTODoesNotDependOnRuntimeTLSPackages(t *testing.T) {
	cfg := &packages.Config{Mode: packages.NeedImports}
	pkgs, err := packages.Load(cfg, ".")
	require.NoError(t, err)
	require.Len(t, pkgs, 1)

	forbidden := []string{
		"github.com/aerospike/aerospike-backup-service/v3/internal/server/tlsconfig",
		"github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret",
		"github.com/aerospike/aerospike-backup-service/v3/pkg/tlsconfig",
	}
	for _, path := range forbidden {
		require.NotContains(t, pkgs[0].Imports, path)
	}
}
