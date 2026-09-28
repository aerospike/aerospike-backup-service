package doccheck_test

import (
	"path/filepath"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/doccheck"
	"github.com/aerospike/aerospike-backup-service/v3/internal/testcatalog"
	"github.com/stretchr/testify/require"
)

// TestIntegrationTestsAreCatalogued keeps test/integration readable as a list of
// scenarios, the promise its README makes.
//
// A reader opens a *_test.go file there to find out what is tested. A helper or a
// container setup in that file is infrastructure they have to read past, so it
// belongs in a non-test file next to it; a test declared in such a file runs but
// is invisible to the README. A Test* method with no doc comment is a
// blank row in the README table the generator builds from those comments. And a
// Test* method on a suite no runner starts compiles, looks tested, and never runs.
func TestIntegrationTestsAreCatalogued(t *testing.T) {
	catalog, err := testcatalog.Read(filepath.Join(doccheck.Root(t), "test", "integration"))
	require.NoError(t, err)
	require.NotEmpty(t, catalog.Suites, "no suites found in test/integration/%s", testcatalog.SuitesFile)

	for _, decl := range catalog.Strays {
		t.Errorf("%s: %s is not a Test* method; move it to a non-_test.go file "+
			"(suites and runners belong in %s)", decl.Pos, decl.Name, testcatalog.SuitesFile)
	}

	for _, decl := range catalog.Misplaced {
		t.Errorf("%s: %s is a test declared in a non-test file, where the README and these "+
			"checks do not see it; move it to a *_test.go file", decl.Pos, decl.Name)
	}

	for _, test := range catalog.Orphans {
		t.Errorf("%s: %s.%s never runs: no suite.Run in %s starts %s",
			test.File, test.Suite, test.Name, testcatalog.SuitesFile, test.Suite)
	}

	for _, suite := range catalog.Suites {
		if suite.Doc == "" {
			t.Errorf("%s: suite %s has no doc comment", testcatalog.SuitesFile, suite.Type)
		}

		for _, test := range suite.Tests {
			if test.Doc == "" {
				t.Errorf("%s: %s has no doc comment; its first paragraph is the README row "+
					"saying what it checks", test.File, test.Name)
			}
		}
	}
}
