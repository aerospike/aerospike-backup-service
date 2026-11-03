package doccheck_test

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/doccheck"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	chartValues = "helm/aerospike-backup-service/values.yaml"
	chartReadme = "helm/aerospike-backup-service/README.md"

	// referenceStart is the values line the commented reference follows.
	referenceStart = "backupServiceConfig: {}"
)

// TestHelmValuesReferenceMatchesConfig checks the commented backupServiceConfig
// reference in the chart's values.yaml against the DTOs it describes.
//
// The reference is a comment, so nothing decodes it, and nothing noticed when
// `storage` lost its name level, `prefer-racks` moved to the cluster and the
// secret agent kept a `tls-ca-file` that no longer existed. Every key in it must
// be a field of the type at that position, and a default it states must be the
// `default:` tag the DTO publishes: a field that publishes one must say so, and
// one that publishes none must not claim one.
//
// Values are placeholders and are not checked; the README examples, which must
// also validate, are covered by TestHelmReadmeExamplesValidate.
func TestHelmValuesReferenceMatchesConfig(t *testing.T) {
	root := doccheck.Root(t)
	text, offset := valuesReference(t, root)

	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(text), &doc), "the reference is not YAML")
	require.Len(t, doc.Content, 1)

	published := map[string]string{}
	for _, tag := range doccheck.DefaultTags(t, root, "pkg/dto") {
		if tag.Value != "" {
			published[tag.ID()] = tag.Value
		}
	}

	w := referenceWalker{t: t, published: published, offset: offset}
	w.walk(doc.Content[0], reflect.TypeFor[dto.Config]())
}

// valuesReference returns the commented block that follows referenceStart with
// the comment markers stripped, and the number of file lines before it, so a
// finding can point at the values file.
func valuesReference(t *testing.T, root string) (string, int) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(root, chartValues))
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	start := slices.Index(lines, referenceStart)
	require.NotEqual(t, -1, start, "%s has no %q line", chartValues, referenceStart)

	var block []string

	for _, line := range lines[start+1:] {
		if !strings.HasPrefix(line, "#") {
			break
		}

		block = append(block, strings.TrimPrefix(strings.TrimPrefix(line, "#"), " "))
	}

	require.NotEmpty(t, block, "%s has no commented reference after %q", chartValues, referenceStart)

	return strings.Join(block, "\n"), start + 1
}

// A yamlField is a struct field reachable by its YAML key, with the type that
// declares it: an inlined field belongs to the embedded type, which is where its
// `default:` tag is read from.
type yamlField struct {
	reflect.StructField
	owner string
}

// yamlFields maps YAML keys to the fields of a struct, inlined structs flattened.
func yamlFields(typ reflect.Type) map[string]yamlField {
	fields := map[string]yamlField{}

	for field := range typ.Fields() {
		name, opts, _ := strings.Cut(field.Tag.Get("yaml"), ",")

		switch {
		case strings.Contains(opts, "inline"):
			maps.Copy(fields, yamlFields(field.Type))
		case name != "" && name != "-":
			fields[name] = yamlField{StructField: field, owner: typ.Name()}
		}
	}

	return fields
}

// defaultClaim matches the default a reference comment states.
var defaultClaim = regexp.MustCompile(`Default: ("[^"]*"|\S+)`)

// literalDefault matches a claimed default that is a value rather than prose
// such as "keep all" or "no limit".
var literalDefault = regexp.MustCompile(`^(?:[0-9.]+|true|false|[A-Z][A-Z0-9_]*)$`)

type referenceWalker struct {
	t         *testing.T
	published map[string]string // DefaultTag.ID() -> published default
	offset    int
}

// walk pairs each node of the reference with the type decoding would give it.
// A scalar where a struct is expected is a placeholder and ends the walk there.
func (w *referenceWalker) walk(node *yaml.Node, typ reflect.Type) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	switch {
	case node.Kind == yaml.SequenceNode && typ.Kind() == reflect.Slice:
		for _, item := range node.Content {
			w.walk(item, typ.Elem())
		}
	case node.Kind == yaml.MappingNode && typ.Kind() == reflect.Map:
		for i := 1; i < len(node.Content); i += 2 {
			w.walk(node.Content[i], typ.Elem())
		}
	case node.Kind == yaml.MappingNode && typ.Kind() == reflect.Struct:
		fields := yamlFields(typ)

		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]

			field, ok := fields[key.Value]
			if !ok {
				w.errorf(key, "%s has no field %q", typ, key.Value)
				continue
			}

			w.checkDefault(key, value, field)
			w.walk(value, field.Type)
		}
	}
}

func (w *referenceWalker) checkDefault(key, value *yaml.Node, field yamlField) {
	var claim string
	if m := defaultClaim.FindStringSubmatch(key.LineComment + " " + value.LineComment); m != nil {
		claim = strings.Trim(m[1], `".,;)`)
	}

	published, ok := w.published[field.owner+"."+field.Name]

	switch {
	case ok && claim == "":
		w.errorf(key, "%s publishes the default %q; the reference does not state it", key.Value, published)
	case ok && claim != published:
		w.errorf(key, "%s publishes the default %q; the reference says %q", key.Value, published, claim)
	case !ok && literalDefault.MatchString(claim):
		w.errorf(key, "%s publishes no default; the reference says %q", key.Value, claim)
	}
}

func (w *referenceWalker) errorf(node *yaml.Node, format string, args ...any) {
	w.t.Helper()
	w.t.Errorf("%s:%d: %s", chartValues, w.offset+node.Line, fmt.Sprintf(format, args...))
}

// yamlFence matches a fenced YAML block in Markdown.
var yamlFence = regexp.MustCompile("(?s)```yaml\n(.*?)```")

// TestHelmReadmeExamplesValidate runs every backupServiceConfig example in the
// chart README through the decoder and the validation the service applies at
// startup, so an example cannot show a configuration that would not start. With
// `-r` in args the value is a remote configuration pointer, a storage.
func TestHelmReadmeExamplesValidate(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(doccheck.Root(t), chartReadme))
	require.NoError(t, err)

	text := string(content)
	found := 0

	for _, m := range yamlFence.FindAllStringSubmatchIndex(text, -1) {
		var values struct {
			Args   []string  `yaml:"args"`
			Config yaml.Node `yaml:"backupServiceConfig"` //nolint:tagliatelle // the Helm value, camelCase
		}

		if err := yaml.Unmarshal([]byte(text[m[2]:m[3]]), &values); err != nil || values.Config.IsZero() {
			continue
		}

		found++
		at := fmt.Sprintf("%s:%d", chartReadme, 1+strings.Count(text[:m[0]], "\n"))

		t.Run(at, func(t *testing.T) {
			encoded, err := yaml.Marshal(&values.Config)
			require.NoError(t, err)

			if slices.Contains(values.Args, "-r") {
				var pointer dto.Storage
				require.NoError(t, decoder.Deserialize(&pointer, bytes.NewReader(encoded), decoder.YAML))
				require.NoError(t, pointer.Validate())

				return
			}

			var config dto.Config
			require.NoError(t, decoder.Deserialize(&config, bytes.NewReader(encoded), decoder.YAML))
			require.NoError(t, config.Validate())
		})
	}

	require.NotZero(t, found, "%s has no backupServiceConfig example", chartReadme)
}
