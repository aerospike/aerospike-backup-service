package doccheck_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

const openapi = "docs/openapi.json"

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
				t.Errorf("%s names the field %q, which %s does not declare", at, name, openapi)
			})
		}
	}
}

// schemaProperties collects every property name the generated schemas declare.
func schemaProperties(t *testing.T) map[string]bool {
	t.Helper()

	root := doccheck.Root(t)
	known := map[string]bool{
		"aerospike-backup-service": true,
	}

	content, err := os.ReadFile(filepath.Join(root, openapi))
	require.NoErrorf(t, err, "read %s", openapi)

	var document any
	require.NoErrorf(t, json.Unmarshal(content, &document), "parse %s", openapi)

	collectProperties(document, known)

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
