package main

import (
	"fmt"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/internal/testcatalog"
)

const integrationTestsDir = "test/integration"

// renderIntegrationTests lists every integration test, one table per suite, from
// the Test* doc comments. Links are relative to integrationTestsDir, where the
// README carrying the tag lives.
func renderIntegrationTests() string {
	catalog, err := testcatalog.Read(integrationTestsDir)
	if err != nil {
		panic(fmt.Errorf("read integration tests: %w", err))
	}

	var b strings.Builder

	b.WriteString("\n")

	for _, suite := range catalog.Suites {
		_, _ = fmt.Fprintf(&b, "\n### %s\n\n", suite.Runner)

		if summary := oneLine(suite.Doc); summary != "" {
			_, _ = fmt.Fprintf(&b, "%s\n\n", summary)
		}

		b.WriteString("| Test | What it checks | Cases |\n|---|---|---|\n")

		for _, test := range suite.Tests {
			_, _ = fmt.Fprintf(&b, "| [`%s`](%s) | %s | %s |\n",
				test.Name, test.File, tableCell(test.Summary()), tableCell(renderCases(test.Cases)))
		}
	}

	b.WriteString("\n")

	return b.String()
}

func renderCases(cases []string) string {
	quoted := make([]string, 0, len(cases))
	for _, name := range cases {
		quoted = append(quoted, "`"+name+"`")
	}

	return strings.Join(quoted, "<br>")
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// tableCell keeps text from breaking out of its Markdown table cell.
func tableCell(text string) string {
	return strings.ReplaceAll(text, "|", `\|`)
}
