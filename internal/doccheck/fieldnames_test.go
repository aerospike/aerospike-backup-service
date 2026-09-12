package doccheck_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/doccheck"
	"github.com/stretchr/testify/require"
)

// fieldToken matches a backticked kebab-case word, which is how the documents
// write a configuration or request field: `socket-timeout`, `source-name`.
var fieldToken = regexp.MustCompile("`([a-z][a-z0-9]*(?:-[a-z0-9]+)+)`")

// nonFieldTokens are backticked kebab-case words that are deliberately not field
// names — a command, a tool, a branch. Add one only after checking that the word
// really is not a field; the usual cause of a failure here is prose naming a
// field that does not exist.
//
//nolint:gochecknoglobals // a table, read by the test below.
var nonFieldTokens = map[string]bool{}

// TestDocumentedFieldNamesExist checks that every field the prose names is real.
//
// The configuration examples are generated from Go structs and the endpoint
// references from the OpenAPI document, so neither can name something that does
// not exist. Sentences around them can, and did: api-examples.md offered
// `storage-name` as an alternative to `storage` when the fields are `source-name`
// and `source`, and migration.md explained `parallel-write` by reference to
// `parallel-read`, which is an internal backup-go field an operator cannot set.
// Both read plausibly; neither would be caught by anything else here.
func TestDocumentedFieldNamesExist(t *testing.T) {
	known := schemaProperties(t)
	require.NotEmpty(t, known, "no properties found; the schema scan is broken, not the docs")

	root := doccheck.Root(t)

	for _, file := range docFiles {
		content, err := os.ReadFile(filepath.Join(root, file))
		require.NoErrorf(t, err, "read %s", file)

		text := string(content)

		for _, match := range fieldToken.FindAllStringSubmatchIndex(text, -1) {
			name := text[match[2]:match[3]]
			if known[name] || nonFieldTokens[name] {
				continue
			}

			at := fmt.Sprintf("%s:%d", file, 1+strings.Count(text[:match[0]], "\n"))

			t.Run(at+"/"+name, func(t *testing.T) {
				t.Errorf("%s names the field %q, which is in neither docs/config.schema.json "+
					"nor docs/openapi.json.\nDid you mean one of: %s?\n"+
					"If it is not a field at all, add it to nonFieldTokens.",
					at, name, strings.Join(similar(name, known), ", "))
			})
		}
	}
}

// schemaProperties collects every property name the generated schemas declare.
func schemaProperties(t *testing.T) map[string]bool {
	t.Helper()

	root := doccheck.Root(t)
	known := make(map[string]bool)

	for _, file := range []string{"docs/config.schema.json", "docs/openapi.json"} {
		content, err := os.ReadFile(filepath.Join(root, file))
		require.NoErrorf(t, err, "read %s", file)

		var document any
		require.NoErrorf(t, json.Unmarshal(content, &document), "parse %s", file)

		collectProperties(document, known)
	}

	return known
}

// collectProperties walks a decoded JSON schema for "properties" objects.
func collectProperties(node any, known map[string]bool) {
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			if key == "properties" {
				if properties, ok := child.(map[string]any); ok {
					for name := range properties {
						known[name] = true
					}
				}
			}

			collectProperties(child, known)
		}
	case []any:
		for _, child := range value {
			collectProperties(child, known)
		}
	}
}

// similar suggests the known fields closest to the one the prose named, which is
// usually enough to see the mistake: `storage-name` next to `source-name`.
//
// Candidates are ranked by edit distance. Alphabetical order would bury the
// answer, and so would a prefix match: a dozen fields end in "-name", and
// "source-name" shares only one letter with "storage-name" that names it wrongly.
func similar(name string, known map[string]bool) []string {
	candidates := make([]string, 0, len(known))
	for candidate := range known {
		candidates = append(candidates, candidate)
	}

	sort.Slice(candidates, func(i, j int) bool {
		left, right := distance(name, candidates[i]), distance(name, candidates[j])
		if left != right {
			return left < right
		}

		return candidates[i] < candidates[j]
	})

	const maxSuggestions = 4
	if len(candidates) > maxSuggestions {
		candidates = candidates[:maxSuggestions]
	}

	return candidates
}

// distance is the Levenshtein distance between two field names.
func distance(a, b string) int {
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)

	for j := range previous {
		previous[j] = j
	}

	for i := 1; i <= len(a); i++ {
		current[0] = i

		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}

			current[j] = min(previous[j]+1, min(current[j-1]+1, previous[j-1]+cost))
		}

		previous, current = current, previous
	}

	return previous[len(b)]
}
