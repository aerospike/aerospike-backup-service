package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRequireRepoRoot covers the mistake the guard exists for: every path the
// generator reads is relative to the repository root, so running it from
// anywhere else used to fail partway through, after some documents had already
// been rewritten.
func TestRequireRepoRoot(t *testing.T) {
	t.Run("accepts the repository root", func(t *testing.T) {
		t.Chdir(filepath.Join("..", ".."))

		assert.NotPanics(t, requireRepoRoot)
	})

	t.Run("rejects a directory with no module", func(t *testing.T) {
		t.Chdir(t.TempDir())

		assert.Panics(t, requireRepoRoot)
	})

	// A nested or unrelated module also has a go.mod, so the file existing is
	// not enough to prove this is the right tree.
	t.Run("rejects another module", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
			[]byte("module example.com/other\n\ngo 1.25\n"), 0600))
		t.Chdir(dir)

		assert.Panics(t, requireRepoRoot)
	})
}

func TestRenderGoVersion_ReadsTheModuleDirective(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))

	version := renderGoVersion()

	assert.Regexp(t, `^1\.\d+`, version, "the go directive should render as a bare version")

	content, err := os.ReadFile("go.mod")
	require.NoError(t, err)
	assert.Contains(t, string(content), "go "+version)
}

// TestRenderFilterExpressions_EncodesEveryExample is the point of generating the
// table: a base64 filter expression cannot be proofread, and one wrong character
// is a routine the service refuses to start on.
func TestRenderFilterExpressions_EncodesEveryExample(t *testing.T) {
	table := renderFilterExpressions()

	require.NotEmpty(t, filterExpressionExamples)

	for _, example := range filterExpressionExamples {
		encoded, err := example.expression.Base64()
		require.NoErrorf(t, err, "encode %q", example.description)
		assert.Containsf(t, table, "| `"+example.description+"` | `"+encoded+"` |",
			"the row for %q should carry the encoded expression", example.description)
	}

	assert.Equal(t, len(filterExpressionExamples)+2,
		strings.Count(strings.TrimSpace(table), "\n")+1,
		"the table should be a header, a separator and one row per example")
}

func TestRenderDuration(t *testing.T) {
	tests := map[string]struct {
		duration time.Duration
		want     string
	}{
		"whole minutes":     {10 * time.Minute, "10 minutes"},
		"whole seconds":     {10 * time.Second, "10 seconds"},
		"whole hours":       {2 * time.Hour, "2 hours"},
		"a single unit":     {time.Minute, "1 minute"},
		"largest unit wins": {120 * time.Minute, "2 hours"},
		// 90s is a whole number of seconds even though it is not a whole number
		// of minutes, so it names the largest unit that divides it exactly.
		"a whole count of the smaller unit": {90 * time.Second, "90 seconds"},
		// Nothing in the project renders one of these today; falling back to the
		// Go form keeps an odd value unambiguous rather than rounding it away.
		"no unit divides it": {90500 * time.Millisecond, "1m30.5s"},
		"sub-second":         {500 * time.Millisecond, "500ms"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.want, renderDuration(test.duration))
		})
	}
}
