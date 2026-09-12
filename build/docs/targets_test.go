package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// docsGenerated captures the value of the Makefile's DOCS_GENERATED, continuation
// lines included.
var docsGenerated = regexp.MustCompile(`(?m)^DOCS_GENERATED\s*:?=((?:[^\n\\]*\\\n)*[^\n]*)`)

// TestTargetFilesAreGuardedByDocsCheck is the one thing that keeps two lists of
// the same files, written in two languages, from drifting apart.
//
// build/docs rewrites every file in targetFiles; "make docs-check" fails CI when
// a file in the Makefile's DOCS_GENERATED differs from what the generator just
// produced. A file in the first list but not the second is rewritten by "make
// docs" and guarded by nothing: a stale generated region would survive in the
// committed file, because CI diffs only the second list. That is not a
// hypothetical — docs/architecture.md sat in exactly that gap.
func TestTargetFilesAreGuardedByDocsCheck(t *testing.T) {
	guarded := makefileDocsGenerated(t)

	for _, path := range targetFiles {
		assert.Containsf(t, guarded, path,
			"%s is rewritten by build/docs but is not in the Makefile's DOCS_GENERATED, "+
				"so \"make docs-check\" would not notice it going stale; add it there",
			path)
	}
}

func makefileDocsGenerated(t *testing.T) []string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	require.NoError(t, err, "read Makefile")

	match := docsGenerated.FindSubmatch(content)
	require.NotNil(t, match, "Makefile has no DOCS_GENERATED assignment")

	return strings.Fields(strings.ReplaceAll(string(match[1]), "\\\n", " "))
}
