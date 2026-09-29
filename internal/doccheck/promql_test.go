package doccheck_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/aerospike/aerospike-backup-service/v3/internal/doccheck"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	metricsFile = "docs/metrics.json"
	// metricPrefix is shared by every metric the service exports. A query naming
	// anything else, such as `up`, is about another exporter.
	metricPrefix = "aerospike_backup_service_"
)

// queryDocFiles are the documents whose queries are checked. The migration guide
// is not one of them: it names metrics as they were in earlier releases,
// including ones since removed.
//
//nolint:gochecknoglobals // a table, read by the test below.
var queryDocFiles = slices.DeleteFunc(slices.Clone(docFiles), func(file string) bool {
	return file == "docs/migration.md"
})

// labelValueTypes maps a label to the Go type whose string constants are its
// values, so a query filtering on a value the service never sets is caught.
// Labels not listed here, such as routine, carry user-chosen names.
//
//nolint:gochecknoglobals // a table, read by the test below.
var labelValueTypes = map[string]struct{ pkgDir, typeName string }{
	"outcome": {"pkg/service/prometheus", "Outcome"},
	"type":    {"pkg/model", "BackupType"},
}

// querySpan matches an inline code span that mentions one of the service metrics.
var querySpan = regexp.MustCompile("`([^`\n]*" + metricPrefix + "[^`\n]*)`")

// TestPromQLMatchesMetrics checks the queries the documents offer against the
// metrics the service exports.
//
// The metrics table is generated, but the queries and alert rules around it are
// copied into dashboards and alert managers. A metric or label renamed in code
// leaves such a query matching no series at all, and an alert built on it never
// fires, with nothing anywhere reporting an error. Each query is parsed as
// PromQL, and every metric, label and known label value it names is looked up in
// docs/metrics.json and the constants the service sets them from.
func TestPromQLMatchesMetrics(t *testing.T) {
	root := doccheck.Root(t)
	metricLabels := exportedMetrics(t, root)
	labelValues := make(map[string][]string)

	for label, source := range labelValueTypes {
		labelValues[label] = doccheck.StringConstants(t, root, source.pkgDir, source.typeName)
		require.NotEmptyf(t, labelValues[label], "no %s constants found in %s", source.typeName, source.pkgDir)
	}

	for _, file := range queryDocFiles {
		content, err := os.ReadFile(filepath.Join(root, file))
		require.NoErrorf(t, err, "read %s", file)

		for _, query := range documentQueries(t, string(content)) {
			t.Run(fmt.Sprintf("%s:%d", file, query.line), func(t *testing.T) {
				checkQuery(t, query.expr, metricLabels, labelValues)
			})
		}
	}
}

type documentQuery struct {
	expr string
	line int
}

// documentQueries returns the inline code spans that mention a service metric
// and the expressions of every prometheus-rules block.
func documentQueries(t *testing.T, content string) []documentQuery {
	t.Helper()

	var queries []documentQuery

	for _, match := range querySpan.FindAllStringSubmatchIndex(content, -1) {
		queries = append(queries, documentQuery{
			expr: content[match[2]:match[3]],
			line: 1 + strings.Count(content[:match[0]], "\n"),
		})
	}

	for _, block := range doccheck.CodeBlocks(content) {
		if block.Args != prometheusRulesArg {
			continue
		}

		for _, rule := range prometheusRules(t, block.Body) {
			queries = append(queries, documentQuery{expr: rule.Expr, line: block.Line})
		}
	}

	return queries
}

func checkQuery(t *testing.T, query string, metricLabels, labelValues map[string][]string) {
	t.Helper()

	expr, err := metricsql.Parse(query)
	require.NoErrorf(t, err, "%q is not a valid query", query)

	metricsql.VisitAll(expr, func(node metricsql.Expr) {
		switch node := node.(type) {
		case *metricsql.MetricExpr:
			for _, filters := range node.LabelFilterss {
				checkSelector(t, filters, metricLabels, labelValues)
			}
		case *metricsql.AggrFuncExpr:
			checkGrouping(t, node.Modifier, metricLabels, node.Args...)
		case *metricsql.BinaryOpExpr:
			checkGrouping(t, node.GroupModifier, metricLabels, node.Left, node.Right)
			checkGrouping(t, node.JoinModifier, metricLabels, node.Left, node.Right)
		}
	})
}

// checkGrouping checks the labels of a by, without, on, ignoring, group_left or
// group_right clause. A clause naming a label that none of the service metrics
// it applies to carries groups everything into one series, silently, so each
// label must belong to at least one of them. A clause over no service metric is
// about another exporter and is left alone.
func checkGrouping(t *testing.T, modifier metricsql.ModifierExpr, metricLabels map[string][]string,
	operands ...metricsql.Expr) {
	t.Helper()

	if len(modifier.Args) == 0 {
		return
	}

	names := exportedMetricsIn(operands, metricLabels)
	if len(names) == 0 {
		return
	}

	for _, label := range modifier.Args {
		carried := slices.ContainsFunc(names, func(name string) bool {
			return slices.Contains(metricLabels[name], label)
		})
		if !carried {
			t.Errorf("%s (%s): none of %v has a label %q", modifier.Op, label, names, label)
		}
	}
}

// exportedMetricsIn returns the exported service metrics the expressions select.
// An unknown one is reported by checkSelector, so it is skipped here.
func exportedMetricsIn(exprs []metricsql.Expr, metricLabels map[string][]string) []string {
	var names []string

	for _, expr := range exprs {
		metricsql.VisitAll(expr, func(node metricsql.Expr) {
			metric, ok := node.(*metricsql.MetricExpr)
			if !ok {
				return
			}

			for _, filters := range metric.LabelFilterss {
				name := metricName(filters)
				if _, exported := metricLabels[name]; exported {
					names = append(names, name)
				}
			}
		})
	}

	return names
}

func checkSelector(t *testing.T, filters []metricsql.LabelFilter, metricLabels, labelValues map[string][]string) {
	t.Helper()

	name := metricName(filters)
	if !strings.HasPrefix(name, metricPrefix) {
		return
	}

	labels, exported := metricLabels[name]
	if !exported {
		t.Errorf("%s is not a metric the service exports (see %s)", name, metricsFile)
		return
	}

	for _, filter := range filters {
		if filter.Label == "__name__" {
			continue
		}

		if !slices.Contains(labels, filter.Label) {
			t.Errorf("%s has no label %q; its labels are %v", name, filter.Label, labels)
			continue
		}

		values, known := labelValues[filter.Label]
		if known && !filter.IsRegexp && !slices.Contains(values, filter.Value) {
			t.Errorf("%s{%s=%q}: the service never sets that value; it sets %v",
				name, filter.Label, filter.Value, values)
		}
	}
}

func metricName(filters []metricsql.LabelFilter) string {
	for _, filter := range filters {
		if filter.Label == "__name__" {
			return filter.Value
		}
	}

	return ""
}

// exportedMetrics maps each metric in docs/metrics.json to its labels.
func exportedMetrics(t *testing.T, root string) map[string][]string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(root, metricsFile))
	require.NoErrorf(t, err, "read %s", metricsFile)

	var metrics []struct {
		Name   string   `json:"name"`
		Labels []string `json:"labels"`
	}
	require.NoErrorf(t, json.Unmarshal(content, &metrics), "parse %s", metricsFile)
	require.NotEmptyf(t, metrics, "%s lists no metrics", metricsFile)

	labels := make(map[string][]string, len(metrics))
	for _, metric := range metrics {
		labels[metric.Name] = metric.Labels
	}

	return labels
}

// prometheusRule is one entry of a Prometheus rule group.
type prometheusRule struct {
	Alert         string            `yaml:"alert"`
	Record        string            `yaml:"record"`
	Expr          string            `yaml:"expr"`
	For           string            `yaml:"for"`
	KeepFiringFor string            `yaml:"keep_firing_for"` //nolint:tagliatelle // the key Prometheus reads
	Labels        map[string]string `yaml:"labels"`
	Annotations   map[string]string `yaml:"annotations"`
}

// prometheusRules decodes a prometheus-rules block, rejecting keys Prometheus
// would reject too.
func prometheusRules(t *testing.T, body string) []prometheusRule {
	t.Helper()

	var rules []prometheusRule

	decoder := yaml.NewDecoder(strings.NewReader(body))
	decoder.KnownFields(true)
	require.NoError(t, decoder.Decode(&rules), "the block is not a list of Prometheus rules")

	for _, rule := range rules {
		require.NotEmpty(t, rule.Expr, "rule %q has no expr", rule.Alert+rule.Record)
		require.NotEqualf(t, rule.Alert == "", rule.Record == "",
			"a rule is either an alert or a recording rule: %+v", rule)
	}

	return rules
}
