package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	servertls "github.com/aerospike/aerospike-backup-service/v3/internal/server/tlsconfig"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	metrics "github.com/aerospike/aerospike-backup-service/v3/pkg/service/prometheus"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v3"
)

const (
	examplesDir = "docs/examples"

	// openAPIRelPath is written earlier in the same run by generateOpenAPI.
	openAPIRelPath = "docs/openapi.json"
)

// targetFiles lists every Markdown file that may contain generated sections
// (DTO examples, the default config block, the metrics table, or the RBAC
// matrix reserved below). Each file is processed independently and markers
// that aren't present in a given file are simply left alone, so a new doc
// (e.g. docs/security.md) only needs to be added here to opt in.
var targetFiles = []string{
	"README.md",
	"docs/installation.md",
	"docs/configuration.md",
	"docs/api-examples.md",
	"docs/architecture.md",
	"docs/monitoring.md",
	"docs/migration.md",
	"docs/security.md",
	"docs/development.md",
}

func generateReadme() {
	// generate markdown dto descriptions from open-api
	generateMarkdownFiles()
	// generate example files from jsonExamples and yamlExamples
	generateExampleFiles()

	// Compute the Prometheus metrics table once; it's applied to every target
	// file below, and only files that actually carry the marker are changed.
	metricRows := extractRows()
	writeMetricsToFile(metricRows)

	// Endpoints come from the OpenAPI document generated earlier in this run.
	renderers := newRenderers(loadEndpoints(), renderMetricsTable(metricRows))

	for _, path := range targetFiles {
		content, err := os.ReadFile(path)
		if err != nil {
			panic(fmt.Errorf("failed to read target file %q: %w", path, err))
		}

		content = applyTags(content, renderers)

		err = os.WriteFile(path, content, 0600)
		if err != nil {
			panic(fmt.Errorf("failed to write target file %q: %w", path, err))
		}
	}
}

// newRenderers collects every tag the documents may use into one namespace.
//
// Three sources contribute: every operation in the OpenAPI document, every worked
// example built from the DTO structs, and a handful of one-off sections. Two
// sources claiming the same id would make a document's meaning depend on map
// iteration order, so that is a build failure rather than a surprise.
func newRenderers(endpoints map[string]endpoint, metricsTable string) map[string]renderer {
	renderers := make(map[string]renderer)

	add := func(source, id string, render renderer) {
		if _, taken := renderers[id]; taken {
			panic(fmt.Errorf("duplicate tag %q: %s claims an id another source already provides",
				id, source))
		}

		renderers[id] = render
	}

	for id := range endpoints {
		add(openAPIRelPath, id, func(args string) string { return renderEndpoint(id, args, endpoints) })
	}

	for id := range jsonExamples {
		add("jsonExamples", id, noArgs(id, func() string { return renderExample(id) }))
	}

	for id := range yamlExamples {
		add("yamlExamples", id, noArgs(id, func() string { return renderExample(id) }))
	}

	defaultConfig := readDefaultConfig()

	add("generator", "DefaultConfig", noArgs("DefaultConfig", func() string {
		return fence("yaml", defaultConfig)
	}))
	add("generator", "Metrics", noArgs("Metrics", func() string { return "\n\n" + metricsTable }))
	add("generator", "TLSReloadInterval", noArgs("TLSReloadInterval", func() string {
		return renderDuration(servertls.WatchInterval)
	}))
	add("generator", "StorageClientCacheTTL", noArgs("StorageClientCacheTTL", func() string {
		return renderDuration(storage.ClientCacheTTL)
	}))
	add("generator", "GoVersion", noArgs("GoVersion", renderGoVersion))
	add("generator", "FilterExpressions", noArgs("FilterExpressions", renderFilterExpressions))
	// Reserved for the security plan: a document may carry the tag before there
	// is anything to put in it.
	add("generator", "RBACMatrix", noArgs("RBACMatrix", func() string { return "" }))

	return renderers
}

func generateExampleFiles() {
	_ = os.RemoveAll(examplesDir)
	_ = os.MkdirAll(examplesDir, 0755)

	for name, example := range jsonExamples {
		fileName := filepath.Join(examplesDir, name+".json")
		fileContent, err := json.MarshalIndent(example, "", "  ")
		if err != nil {
			panic(fmt.Errorf("failed to marshal json example %q: %w", name, err))
		}
		fileContent = fmt.Appendln(fileContent, "")
		err = os.WriteFile(fileName, fileContent, 0600)
		if err != nil {
			panic(fmt.Errorf("failed to write json example file %q: %w", fileName, err))
		}
	}

	for name, example := range yamlExamples {
		fileName := filepath.Join(examplesDir, name+".yaml")
		fileContent, err := marshalYAML(example)
		if err != nil {
			panic(fmt.Errorf("failed to marshal yaml example %q: %w", name, err))
		}
		err = os.WriteFile(fileName, fileContent, 0600)
		if err != nil {
			panic(fmt.Errorf("failed to write yaml example file %q: %w", fileName, err))
		}
	}
}

// apiDocsBaseURL is where the published API browser serves an operation.
const apiDocsBaseURL = "https://aerospike.github.io/aerospike-backup-service/#/"

// linkFlag asks for the linked call-out form rather than a bare code span.
const linkFlag = "link"

// endpoint is one operation as the published API browser presents it.
type endpoint struct {
	method string
	path   string
	tag    string
}

// renderEndpoint expands one region, keeping its markers so the next run finds it.
func renderEndpoint(operationID, args string, endpoints map[string]endpoint) string {
	asLink, query := parseEndpointArgs(operationID, args)

	operation, found := endpoints[operationID]
	if !found {
		panic(fmt.Errorf("unknown operation %q: no such operationId in %s", operationID, openAPIRelPath))
	}

	// A call-out on its own line addresses the service through the {{baseUrl}}
	// placeholder the API examples use, and links to the published browser.
	if asLink {
		return fmt.Sprintf("\n[`%s {{baseUrl}}%s%s`](%s%s/%s)\n",
			operation.method, operation.path, query,
			apiDocsBaseURL, operation.tag, operationID)
	}

	// A mention inside a sentence is just the method and path.
	return fmt.Sprintf("`%s %s%s`", operation.method, operation.path, query)
}

// parseEndpointArgs reads the "[link] [?query]" that may follow an operation id.
func parseEndpointArgs(operationID, args string) (asLink bool, query string) {
	for _, field := range strings.Fields(args) {
		switch {
		case field == linkFlag:
			asLink = true
		case strings.HasPrefix(field, "?"):
			query = field
		default:
			panic(fmt.Errorf("tag %q: unknown argument %q", operationID, field))
		}
	}

	return asLink, query
}

// loadEndpoints indexes the generated OpenAPI document by operation id.
func loadEndpoints() map[string]endpoint {
	content, err := os.ReadFile(openAPIRelPath)
	if err != nil {
		panic(fmt.Errorf("failed to read %s: %w", openAPIRelPath, err))
	}

	var document struct {
		Paths map[string]map[string]struct {
			OperationID string   `json:"operationId"`
			Tags        []string `json:"tags"`
		} `json:"paths"`
	}

	if err := json.Unmarshal(content, &document); err != nil {
		panic(fmt.Errorf("failed to parse %s: %w", openAPIRelPath, err))
	}

	endpoints := make(map[string]endpoint)

	for path, operations := range document.Paths {
		for method, operation := range operations {
			if operation.OperationID == "" || len(operation.Tags) == 0 {
				continue
			}

			endpoints[operation.OperationID] = endpoint{
				method: strings.ToUpper(method),
				path:   path,
				tag:    operation.Tags[0],
			}
		}
	}

	return endpoints
}

// MarshalYAML marshals the input into YAML and replaces 4-space indents with 2-space indents.
// we need this to be in sync with Goland's markdown formatter.
func marshalYAML(v any) ([]byte, error) {
	rawYAML, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}

	formattedYAML := strings.ReplaceAll(string(rawYAML), "    ", "  ")

	return []byte(formattedYAML), nil
}

type MetricRow struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Labels      []string `json:"labels"`
	Deprecated  bool     `json:"deprecated"`
}

// renderMetricsTable generates a Markdown table from a list of Prometheus collectors.
func renderMetricsTable(rows []MetricRow) string {
	maxName := len("Name")
	maxType := len("Type")
	maxHelp := len("Description")
	maxLabels := len("Labels")
	for _, r := range rows {
		if len(r.Name) > maxName {
			maxName = len(r.Name)
		}
		if len(r.Type) > maxType {
			maxType = len(r.Type)
		}
		if len(r.Description) > maxHelp {
			maxHelp = len(r.Description)
		}
		labelsStr := strings.Join(r.Labels, ", ")
		if len(labelsStr) > maxLabels {
			maxLabels = len(labelsStr)
		}
	}

	// Adding 2 for the backticks `` around the name
	const quotes = 2

	var sb strings.Builder
	// Header
	_, _ = fmt.Fprintf(&sb, "| %-*s | %-*s | %-*s | %-*s |\n",
		maxName+quotes, "Name", maxType, "Type", maxHelp, "Description", maxLabels, "Labels")
	// Separator
	_, _ = fmt.Fprintf(&sb, "|-%s-|-%s-|-%s-|-%s-|\n",
		strings.Repeat("-", maxName+quotes),
		strings.Repeat("-", maxType),
		strings.Repeat("-", maxHelp),
		strings.Repeat("-", maxLabels))
	// Body
	for _, r := range rows {
		name := "`" + r.Name + "`"
		labelsStr := strings.Join(r.Labels, ", ")
		_, _ = fmt.Fprintf(&sb, "| %-*s | %-*s | %-*s | %-*s |\n",
			maxName+quotes, name, maxType, r.Type, maxHelp, r.Description, maxLabels, labelsStr)
	}

	return sb.String()
}

func extractRows() []MetricRow {
	var rows []MetricRow
	// This regex extracts the name, help text, and variable labels from the
	// description string of a Prometheus metric.
	//nolint:lll
	prometheusRE := regexp.MustCompile(
		`Desc{fqName:\s*"([^"]+)",\s*help:\s*"([^"]+)",\s*unit:\s*"[^"]*",\s*constLabels:\s*{[^}]*},\s*variableLabels:\s*{([^}]*)}}`)

	// Iterate over all registered metrics.
	for _, metric := range metrics.AllMetrics {
		ch := make(chan *prometheus.Desc, 1)
		metric.Describe(ch)
		close(ch)
		for desc := range ch {
			str := desc.String()
			matches := prometheusRE.FindStringSubmatch(str)
			if len(matches) != 4 {
				panic("Failed to match Prometheus description: " + str)
			}
			helpText := matches[2]
			deprecated := strings.Contains(helpText, "(Deprecated")

			var labels []string
			if matches[3] != "" {
				labels = strings.Split(matches[3], ",")
			}
			rows = append(rows, MetricRow{matches[1], metricsType(metric), helpText, labels, deprecated})
		}
	}

	// Sort rows to have non-deprecated metrics on top.
	slices.SortFunc(rows, func(a, b MetricRow) int {
		if a.Deprecated != b.Deprecated {
			if a.Deprecated {
				return 1
			}

			return -1
		}

		return cmp.Compare(a.Name, b.Name)
	})

	return rows
}

func writeMetricsToFile(rows []MetricRow) {
	// write metrics to json file
	jsonBytes, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("docs/metrics.json", jsonBytes, 0600)
	if err != nil {
		panic(err)
	}
}

func metricsType(metric prometheus.Collector) string {
	switch metric.(type) {
	case *prometheus.CounterVec:
		return "Counter"
	case *prometheus.GaugeVec:
		return "Gauge"
	case *prometheus.HistogramVec:
		return "Histogram"
	case *prometheus.SummaryVec:
		return "Summary"
	default:
		// Readme generator only; panic is acceptable if an unknown metric type is registered.
		panic(fmt.Sprintf("Unknown metric type %v", metric))
	}
}

// readDefaultConfig loads the packaged configuration that the documentation shows
// as its worked example, and refuses to publish one the service would not accept.
func readDefaultConfig() []byte {
	content, err := os.ReadFile("build/package/config/aerospike-backup-service.yml")
	if err != nil {
		panic(fmt.Errorf("failed to read config YAML: %w", err))
	}

	config, err := dto.NewConfigFromReader(bytes.NewReader(content), decoder.YAML)
	if err != nil {
		panic(fmt.Errorf("failed to parse default config YAML: %w", err))
	}

	if err = config.Validate(); err != nil {
		panic(fmt.Errorf("failed to validate default config YAML: %w", err))
	}

	return content
}
