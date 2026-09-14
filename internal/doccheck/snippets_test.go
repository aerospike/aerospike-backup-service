package doccheck_test

import (
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/doccheck"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// configKeys are the top-level keys of dto.Config. A YAML block is treated as a
// configuration — and therefore decoded — when every key it declares is one of
// these. Anything else (a Prometheus alert rule, a fragment showing one storage
// block, a snippet of generated DTO documentation) is left alone, so the check
// reports real breakage rather than its own guesswork.
//
//nolint:gochecknoglobals // a table, read by the tests below.
var configKeys = map[string]bool{
	"service":            true,
	"aerospike-clusters": true,
	"storage":            true,
	"backup-policies":    true,
	"backup-routines":    true,
	"secret-agents":      true,
}

// TestDocumentedConfigSnippetsDecode runs every configuration example in the
// documentation through the same strict decoder the service uses at startup.
//
// Strict decoding is what makes this worth doing: an unknown field is an error,
// not a warning, so a snippet that still shows a field removed two releases ago
// does not merely mislead — it stops the service from starting. The reader has
// no way to know that until they try it.
func TestDocumentedConfigSnippetsDecode(t *testing.T) {
	root := doccheck.Root(t)

	snippets := doccheck.Snippets(t, root, docFiles, "yaml")
	require.NotEmpty(t, snippets, "no YAML snippets found; the extractor is broken, not the docs")

	checked := 0

	for _, snippet := range snippets {
		// A generated block is rendered from real dto.Config values and verified
		// by "make docs-check"; decoding it here would only test the generator.
		if snippet.Generated || !isConfigSnippet(t, snippet.Body) {
			continue
		}

		checked++

		t.Run(snippet.At(), func(t *testing.T) {
			_, err := dto.NewConfigFromReader(strings.NewReader(snippet.Body), decoder.YAML)
			require.NoErrorf(t, err,
				"the configuration example at %s does not decode; a reader who copies it "+
					"cannot start the service", snippet.At())
		})
	}

	require.Positivef(t, checked,
		"none of the %d YAML snippets looked like a configuration; the classifier is broken",
		len(snippets))
}

// isConfigSnippet reports whether the block is a whole configuration document
// rather than a fragment or an unrelated YAML example.
func isConfigSnippet(t *testing.T, body string) bool {
	t.Helper()

	var document map[string]any
	if err := yaml.Unmarshal([]byte(body), &document); err != nil {
		return false // not YAML at all, or a fragment that does not parse alone.
	}

	if len(document) == 0 {
		return false
	}

	for key := range document {
		if !configKeys[key] {
			return false
		}
	}

	return true
}

// TestGeneratedBlocksAreRecognized is a guard on the check above rather than on
// the documentation.
//
// The generated-block exemption is silent by construction: when it stops working,
// every check simply does more than it should, and nothing fails. It did stop
// working — build/docs collapsed its markers to the single <!-- tag id --> form
// and the pattern here still matched the old one, so for a while no block was
// recognized as generated and the packaged configuration was being re-decoded as
// if a person had typed it. This asserts both directions, so the next change to
// the marker syntax breaks a test instead of quietly widening the check.
func TestGeneratedBlocksAreRecognized(t *testing.T) {
	snippets := doccheck.Snippets(t, doccheck.Root(t), docFiles, "yaml")

	var generated, handWritten int

	for _, snippet := range snippets {
		if snippet.Generated {
			generated++
		} else {
			handWritten++
		}
	}

	require.Positive(t, generated,
		"no YAML block was recognized as generated; build/docs writes <!-- tag id --> "+
			"above every block it renders, so the marker pattern in doccheck.go is stale")
	require.Positive(t, handWritten,
		"every YAML block was treated as generated; the marker pattern matches too much "+
			"and the hand-written examples are no longer checked")
}
