package doccheck_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/doccheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Go toolchain version is enforced in one place, the `go` directive in go.mod,
// and copied into two Docker defaults: the Dockerfile ARG, which the docker-compose
// dev stack and a plain `docker build` use as is, and the docker-bake.hcl variable.
// Every automated build overrides both from go.mod, so a stale default is invisible
// until someone builds without the override and the builder image is older than the
// module requires.
func TestDockerGoVersionMatchesGoMod(t *testing.T) {
	t.Parallel()

	root := doccheck.Root(t)
	want := firstMatch(t, filepath.Join(root, "go.mod"), `(?m)^go (\S+)$`)

	for file, pattern := range map[string]string{
		"Dockerfile":                         `(?m)^ARG GO_VERSION=(\S+)$`,
		"build/docker-build/docker-bake.hcl": `(?s)variable GO_VERSION \{\s*default = "([^"]+)"`,
	} {
		got := firstMatch(t, filepath.Join(root, file), pattern)
		assert.Equal(t, want, got, "%s pins a Go version that differs from go.mod", file)
	}
}

func firstMatch(t *testing.T, path, pattern string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	match := regexp.MustCompile(pattern).FindSubmatch(content)
	require.NotNil(t, match, "%s has no Go version", path)

	return string(match[1])
}
