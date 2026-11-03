package doccheck_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/doccheck"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The Helm chart's values.yaml carries a commented reference of the whole
// service configuration under `backupServiceConfig: {}`. It is the first thing a
// chart user copies, and for a long time nothing read it: it showed `storage`
// without the storage-name level, `prefer-racks` on a routine, a `no-bins`
// option that never existed, and defaults the service does not apply (see
// BKRS-397). The tests below read the block the way a user would and hold it to
// the same sources as the rest of the documentation.

const (
	helmValues     = "helm/aerospike-backup-service/values.yaml"
	helmReadme     = "helm/aerospike-backup-service/README.md"
	configSchema   = "docs/config.schema.json"
	helmBlockStart = "backupServiceConfig: {}"
)

// referenceLine is one line of the commented configuration reference.
type referenceLine struct {
	// number is the 1-based line number in values.yaml.
	number int
	// indent is the column of the key, after the comment marker and any "- ".
	indent int
	// item is true when the line starts a list item ("- key: ...").
	item bool
	// key is the YAML key; placeholders such as <storage-name> are kept verbatim.
	key string
	// hasValue is true when the key is followed by a scalar or flow value.
	hasValue bool
	// comment is the trailing "# ..." text, without the marker.
	comment string
}

var (
	// referenceKey matches "key: value  # comment" after the comment marker is removed.
	referenceKey = regexp.MustCompile(`^(\s*)(- )?([a-z0-9-]+|<[^>]+>):\s*(.*)$`)
	// placeholderKey matches a key written as a placeholder.
	placeholderKey = regexp.MustCompile(`^<[^>]+>$`)
)

// TestHelmConfigReferenceDecodes runs the values.yaml reference through the
// service's strict decoder, so a key the service would reject at startup fails
// here instead. Values are replaced by null: the reference documents shapes and
// keys, not working values, and the placeholders would not parse as numbers or
// booleans anyway.
func TestHelmConfigReferenceDecodes(t *testing.T) {
	lines := helmReference(t)

	var doc strings.Builder

	for _, line := range lines {
		key := line.key
		if placeholderKey.MatchString(key) {
			key = "name-" + strings.Trim(key, "<>")
		}

		prefix := strings.Repeat(" ", line.indent)
		if line.item {
			prefix = strings.Repeat(" ", line.indent-2) + "- "
		}

		value := ""
		if line.hasValue {
			value = " null"
		}

		fmt.Fprintf(&doc, "%s%s:%s\n", prefix, key, value)
	}

	_, err := dto.NewFromReader[dto.Config](strings.NewReader(doc.String()), decoder.YAML)
	require.NoErrorf(t, err, "the configuration reference in %s does not decode:\n%s", helmValues, doc.String())
}

// TestHelmReadmeExamplesValidate loads every backupServiceConfig example in the
// chart README the way the service loads its configuration file: the strict
// decoder, Validate(), then ToModel(), which resolves the references between
// sections (a routine's storage, cluster and policy). So an example a user
// copies is one the service accepts. An example whose args pass
// -r holds a remote-configuration pointer rather than the configuration itself,
// and is checked as the dto.Storage the service reads it into.
func TestHelmReadmeExamplesValidate(t *testing.T) {
	examples := readmeConfigExamples(t)
	require.NotEmptyf(t, examples, "no backupServiceConfig examples found in %s; the scan is broken", helmReadme)

	for _, example := range examples {
		t.Run(example.at, func(t *testing.T) {
			if example.remote {
				storage, err := dto.NewFromReader[dto.Storage](strings.NewReader(example.config), decoder.YAML)
				require.NoErrorf(t, err, "%s: remote-configuration pointer does not decode:\n%s", example.at, example.config)
				require.NoErrorf(t, storage.Validate(), "%s: remote-configuration pointer is invalid", example.at)

				return
			}

			config, err := dto.NewFromReader[dto.Config](strings.NewReader(example.config), decoder.YAML)
			require.NoErrorf(t, err, "%s: example does not decode:\n%s", example.at, example.config)
			require.NoErrorf(t, config.Validate(), "%s: example is invalid", example.at)

			_, err = config.ToModel()
			require.NoErrorf(t, err, "%s: example does not convert to the service model", example.at)
		})
	}
}

// readmeExample is one backupServiceConfig taken from a yaml block in the README.
type readmeExample struct {
	// at is "file:line" of the block.
	at string
	// config is the backupServiceConfig value, as YAML.
	config string
	// remote is true when the block's args pass -r.
	remote bool
}

var yamlBlock = regexp.MustCompile("(?s)```yaml\n(.*?)```")

// readmeConfigExamples returns the backupServiceConfig of every yaml block in
// the chart README that sets one.
func readmeConfigExamples(t *testing.T) []readmeExample {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(doccheck.Root(t), helmReadme))
	require.NoErrorf(t, err, "read %s", helmReadme)

	text := string(content)

	var examples []readmeExample

	for _, match := range yamlBlock.FindAllStringSubmatchIndex(text, -1) {
		at := fmt.Sprintf("%s:%d", helmReadme, 1+strings.Count(text[:match[0]], "\n"))

		var values struct {
			Args   []string  `yaml:"args"`
			Config yaml.Node `yaml:"backupServiceConfig"` //nolint:tagliatelle // the chart's own camelCase value name
		}

		require.NoErrorf(t, yaml.Unmarshal([]byte(text[match[2]:match[3]]), &values), "%s: not valid YAML", at)

		if values.Config.Kind == 0 {
			continue
		}

		config, err := yaml.Marshal(&values.Config)
		require.NoErrorf(t, err, "%s: re-encode backupServiceConfig", at)

		examples = append(examples, readmeExample{
			at:     at,
			config: string(config),
			remote: slices.Contains(values.Args, "-r") || slices.Contains(values.Args, "--remote"),
		})
	}

	return examples
}

// TestHelmConfigReferenceMatchesSchema checks each documented field against the
// generated configuration schema, which publishes the `default:` and `validate:`
// tags of the DTOs (and TestDefaultTagsMatchAppliedDefaults ties those tags to
// the code that applies them):
//   - a stated "Default: X" must be the published default;
//   - a field with a published default must state it;
//   - a field the schema requires must not be marked "Optional".
func TestHelmConfigReferenceMatchesSchema(t *testing.T) {
	schema := loadConfigSchema(t)
	lines := helmReference(t)

	type frame struct {
		indent int
		node   map[string]any
	}

	stack := []frame{{indent: -1, node: schema.root}}

	for _, line := range lines {
		// A list item opens at its dash, two columns left of its first key; the
		// keys that follow at the key's column belong to the same element.
		opens := line.indent
		if line.item {
			opens -= 2
		}

		for len(stack) > 1 && stack[len(stack)-1].indent >= opens {
			stack = stack[:len(stack)-1]
		}

		if line.item {
			stack = append(stack, frame{indent: opens, node: schema.items(stack[len(stack)-1].node)})
		}

		parent := stack[len(stack)-1].node

		at := fmt.Sprintf("%s:%d", helmValues, line.number)

		field, required := schema.child(parent, line.key)
		if field == nil {
			t.Errorf("%s: %q is not a field the schema declares here", at, line.key)

			continue
		}

		checkDocumentedField(t, at, line, field, required)
		stack = append(stack, frame{indent: line.indent, node: field})
	}
}

// documentedDefault matches the "Default: X" note, X being its first word.
var documentedDefault = regexp.MustCompile(`(?i)\bdefault:\s*("[^"]*"|\S+)`)

func checkDocumentedField(t *testing.T, at string, line referenceLine, field map[string]any, required bool) {
	t.Helper()

	comment := strings.TrimSpace(line.comment)

	if required && strings.HasPrefix(strings.ToLower(comment), "optional") {
		t.Errorf("%s: %q is marked Optional, but the schema requires it", at, line.key)
	}

	published, hasDefault := field["default"]
	if !hasDefault || published == nil {
		return
	}

	match := documentedDefault.FindStringSubmatch(comment)
	if match == nil {
		t.Errorf("%s: %q has the published default %v, which the comment does not state", at, line.key, published)

		return
	}

	stated := strings.Trim(strings.TrimRight(match[1], ".,;"), `"`)
	if want := formatDefault(published); stated != want {
		t.Errorf("%s: %q documents the default %q, but the published default is %q", at, line.key, stated, want)
	}
}

// formatDefault renders a JSON default the way the comments write it: numbers
// decode as float64, so 52428800 must not come out as 5.24288e+07.
func formatDefault(value any) string {
	if number, ok := value.(float64); ok {
		return strconv.FormatFloat(number, 'f', -1, 64)
	}

	return fmt.Sprint(value)
}

// helmReference returns the commented configuration reference from values.yaml.
func helmReference(t *testing.T) []referenceLine {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(doccheck.Root(t), helmValues))
	require.NoErrorf(t, err, "read %s", helmValues)

	all := strings.Split(string(content), "\n")

	start := -1

	for i, line := range all {
		if line == helmBlockStart {
			start = i + 1

			break
		}
	}

	require.GreaterOrEqualf(t, start, 0, "%s has no %q line; the reference moved", helmValues, helmBlockStart)

	var lines []referenceLine

	for i := start; i < len(all) && strings.HasPrefix(all[i], "#"); i++ {
		text := strings.TrimPrefix(strings.TrimPrefix(all[i], "#"), " ")
		if strings.TrimSpace(text) == "" {
			continue
		}

		body, comment, _ := strings.Cut(text, " #")

		match := referenceKey.FindStringSubmatch(body)
		require.NotNilf(t, match, "%s:%d is not a key line: %q", helmValues, i+1, all[i])

		indent := len(match[1])
		if match[2] != "" {
			indent += len(match[2])
		}

		lines = append(lines, referenceLine{
			number:   i + 1,
			indent:   indent,
			item:     match[2] != "",
			key:      match[3],
			hasValue: strings.TrimSpace(match[4]) != "",
			comment:  comment,
		})
	}

	require.NotEmptyf(t, lines, "no configuration reference found after %q in %s", helmBlockStart, helmValues)

	return lines
}

// configSchemaDoc navigates docs/config.schema.json.
type configSchemaDoc struct {
	root    map[string]any
	schemas map[string]any
}

func loadConfigSchema(t *testing.T) configSchemaDoc {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(doccheck.Root(t), configSchema))
	require.NoErrorf(t, err, "read %s", configSchema)

	var root map[string]any
	require.NoErrorf(t, json.Unmarshal(content, &root), "parse %s", configSchema)

	components, _ := root["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	require.NotEmptyf(t, schemas, "%s has no components.schemas", configSchema)

	return configSchemaDoc{root: root, schemas: schemas}
}

// resolve follows $ref and single-entry allOf wrappers to the schema they name,
// keeping the "default" of the wrapper when it carries one.
func (s configSchemaDoc) resolve(node map[string]any) map[string]any {
	for node != nil {
		if ref, ok := node["$ref"].(string); ok {
			target, _ := s.schemas[strings.TrimPrefix(ref, "#/components/schemas/")].(map[string]any)
			node = withDefault(target, node)

			continue
		}

		if all, ok := node["allOf"].([]any); ok && len(all) == 1 {
			inner, _ := all[0].(map[string]any)
			node = withDefault(inner, node)

			continue
		}

		return node
	}

	return nil
}

// withDefault returns target, carrying over the wrapper's default if it has one.
func withDefault(target, wrapper map[string]any) map[string]any {
	value, ok := wrapper["default"]
	if !ok || target == nil {
		return target
	}

	merged := make(map[string]any, len(target)+1)
	maps.Copy(merged, target)

	merged["default"] = value

	return merged
}

// items returns the element schema of an array.
func (s configSchemaDoc) items(node map[string]any) map[string]any {
	node = s.resolve(node)
	if node == nil {
		return nil
	}

	element, _ := node["items"].(map[string]any)

	return s.resolve(element)
}

// child returns the schema of key under node, and whether node requires it. A
// placeholder key selects the value schema of a map (additionalProperties).
func (s configSchemaDoc) child(node map[string]any, key string) (map[string]any, bool) {
	node = s.resolve(node)
	if node == nil {
		return nil, false
	}

	if placeholderKey.MatchString(key) {
		value, _ := node["additionalProperties"].(map[string]any)

		return s.resolve(value), false
	}

	properties, _ := node["properties"].(map[string]any)

	field, ok := properties[key].(map[string]any)
	if !ok {
		return nil, false
	}

	required := false

	if list, ok := node["required"].([]any); ok {
		for _, name := range list {
			if name == key {
				required = true
			}
		}
	}

	return s.resolve(field), required
}
