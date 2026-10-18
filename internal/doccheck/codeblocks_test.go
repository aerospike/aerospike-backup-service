package doccheck_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/doccheck"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/stretchr/testify/require"
)

// Info-string arguments that say how a hand-written YAML or JSON block is checked.
const (
	// dtoArg decodes the block into the named DTO: ```yaml dto=Config.
	dtoArg = "dto="
	// prometheusRulesArg reads the block as Prometheus alerting or recording
	// rules, whose expressions TestPromQLMatchesMetrics checks.
	prometheusRulesArg = "prometheus-rules"
	// uncheckedArg opts a block out, for text that is deliberately not current:
	// a schema as it looked in an earlier release, in the migration guide.
	uncheckedArg = "unchecked"
)

// dtoTypes are the DTOs a block may name with dto=. Add one when a document needs
// it; an unknown name fails the test rather than going unchecked.
//
//nolint:gochecknoglobals // a table, read by the test below.
var dtoTypes = map[string]func() any{
	"Config": func() any { return &dto.Config{} },
}

// TestCodeBlocksAreChecked holds every YAML and JSON block in the documents to
// something that fails when the code moves on.
//
// A generated block is rewritten from the Go structs on every run. A hand-written
// one is only as current as the last person who read it, and a field renamed in
// the DTOs leaves it describing a configuration the service refuses to start on.
// So a hand-written block has to say what it is, and it is put through the
// service's own strict decoder, the one that rejects unknown fields. Decoding is
// all it gets: a snippet shows a few fields, not a configuration that validates.
//
// Every block must also name its language, which is what the check keys on; an
// untagged fence would slip past it.
func TestCodeBlocksAreChecked(t *testing.T) {
	root := doccheck.Root(t)

	for _, file := range docFiles {
		content, err := os.ReadFile(filepath.Join(root, file))
		require.NoErrorf(t, err, "read %s", file)

		for _, block := range doccheck.CodeBlocks(string(content)) {
			at := fmt.Sprintf("%s:%d", file, block.Line)

			t.Run(at, func(t *testing.T) {
				checkCodeBlock(t, block)
			})
		}
	}
}

func checkCodeBlock(t *testing.T, block doccheck.CodeBlock) {
	t.Helper()

	if block.Language == "" {
		t.Errorf("code block has no language; name one (text, console, yaml, ...)")
		return
	}

	format, structured := map[string]decoder.SerializationFormat{
		"yaml": decoder.YAML,
		"json": decoder.JSON,
	}[block.Language]
	if !structured || block.Generated {
		return
	}

	switch {
	case strings.HasPrefix(block.Args, dtoArg):
		name := strings.TrimPrefix(block.Args, dtoArg)

		newTarget, known := dtoTypes[name]
		if !known {
			t.Errorf("dto=%s is not in dtoTypes", name)
			return
		}

		err := decoder.Deserialize(newTarget(), strings.NewReader(block.Body), format)
		require.NoErrorf(t, err, "the block does not decode into dto.%s", name)
	case block.Args == prometheusRulesArg:
		prometheusRules(t, block.Body)
	case block.Args == uncheckedArg:
	default:
		t.Errorf("hand-written %s block does not say how it is checked; generate it with a "+
			"<!-- tag --> or mark it ```%s %s<Type>, ```%s %s or ```%s %s",
			block.Language, block.Language, dtoArg, block.Language, prometheusRulesArg,
			block.Language, uncheckedArg)
	}
}
