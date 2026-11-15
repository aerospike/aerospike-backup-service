package testcatalog_test

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/testcatalog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRead(t *testing.T) {
	t.Run("suites and tests", func(t *testing.T) {
		catalog, err := testcatalog.Read("testdata/valid")
		require.NoError(t, err)

		assert.Empty(t, catalog.Strays)
		assert.Empty(t, catalog.Orphans)
		assert.Equal(t, []testcatalog.Suite{
			{
				Type:  "SecondSuite",
				Doc:   "SecondSuite is run second.\n",
				Tests: []testcatalog.Test{{Suite: "SecondSuite", Name: "TestBeta", File: "a_test.go"}},
			},
			{
				Type: "FirstSuite",
				Doc:  "FirstSuite is run first.\n",
				Tests: []testcatalog.Test{
					{
						Suite: "FirstSuite",
						Name:  "TestAlpha",
						File:  "a_test.go",
						Doc:   "TestAlpha checks the first thing.\n\nA second paragraph the summary leaves out.\n",
					},
					{
						Suite: "FirstSuite",
						Name:  "TestGamma",
						File:  "b_test.go",
						Doc:   "TestGamma checks\na thing across lines.\n",
					},
				},
			},
		}, catalog.Suites)
	})

	t.Run("strays and orphans", func(t *testing.T) {
		catalog, err := testcatalog.Read("testdata/violations")
		require.NoError(t, err)

		assert.Equal(t, []testcatalog.Decl{
			{Pos: "a_test.go:3", Name: "bucket"},
			{Pos: "a_test.go:5", Name: "helperState"},
			{Pos: "a_test.go:7", Name: "helper"},
			{Pos: "a_test.go:9", Name: "RunSuite.startThing"},
			{Pos: "suite_test.go:15", Name: "RunSuite.SetupTest"},
		}, catalog.Strays)
		assert.Equal(t, []testcatalog.Test{
			{Suite: "NobodySuite", Name: "TestNeverRuns", File: "a_test.go"},
		}, catalog.Orphans)
		require.Len(t, catalog.Suites, 1)
		assert.Len(t, catalog.Suites[0].Tests, 1)
	})

	t.Run("missing directory reads as empty", func(t *testing.T) {
		catalog, err := testcatalog.Read("testdata/absent")
		require.NoError(t, err)
		assert.Empty(t, catalog.Suites)
	})
}
