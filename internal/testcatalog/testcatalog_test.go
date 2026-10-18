package testcatalog_test

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/testcatalog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRead(t *testing.T) {
	t.Run("suites, tests and cases", func(t *testing.T) {
		catalog, err := testcatalog.Read("testdata/valid")
		require.NoError(t, err)

		assert.Empty(t, catalog.Strays)
		assert.Empty(t, catalog.Orphans)
		assert.Equal(t, []testcatalog.Suite{
			{
				Type:   "SecondSuite",
				Runner: "TestSecond",
				Doc:    "SecondSuite is run second.\n",
				Tests:  []testcatalog.Test{{Suite: "SecondSuite", Name: "TestBeta", File: "a_test.go"}},
			},
			{
				Type:   "FirstSuite",
				Runner: "TestFirst",
				Doc:    "FirstSuite is run first.\n",
				Tests: []testcatalog.Test{
					{
						Suite: "FirstSuite",
						Name:  "TestAlpha",
						File:  "a_test.go",
						Doc:   "TestAlpha checks the first thing.\n\nA second paragraph the summary leaves out.\n",
						Cases: []string{"one", "two"},
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

func TestTestSummary(t *testing.T) {
	tests := []struct {
		name string
		test testcatalog.Test
		want string
	}{
		{
			name: "drops the leading name and keeps the first paragraph",
			test: testcatalog.Test{Name: "TestAlpha", Doc: "TestAlpha checks\nthe thing.\n\nMore.\n"},
			want: "Checks the thing.",
		},
		{
			name: "keeps a doc that does not start with the name",
			test: testcatalog.Test{Name: "TestAlpha", Doc: "Checks the thing.\n"},
			want: "Checks the thing.",
		},
		{
			name: "empty doc",
			test: testcatalog.Test{Name: "TestAlpha"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.test.Summary())
		})
	}
}
