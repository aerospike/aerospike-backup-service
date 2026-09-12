package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	as "github.com/aerospike/aerospike-client-go/v8"
)

// renderDuration writes a duration the way the sentence around it reads.
//
// time.Duration.String is built for logs: 10 * time.Minute prints as "10m0s",
// which is noise in a sentence about how long a credential rotation takes. Whole
// units get their English name; anything else falls back to the Go form, which is
// at least unambiguous.
func renderDuration(d time.Duration) string {
	for _, unit := range []struct {
		size time.Duration
		name string
	}{
		{time.Hour, "hour"},
		{time.Minute, "minute"},
		{time.Second, "second"},
	} {
		if d < unit.size || d%unit.size != 0 {
			continue
		}

		count := int64(d / unit.size)
		if count == 1 {
			return "1 " + unit.name
		}

		return fmt.Sprintf("%d %ss", count, unit.name)
	}

	return d.String()
}

// goDirective matches the `go` line of a module file, which is the one place the
// project states the toolchain version it is built with.
var goDirective = regexp.MustCompile(`(?m)^go\s+(\S+)\s*$`)

// renderGoVersion reads the required Go version out of go.mod.
//
// Every document that lists build prerequisites needs this number, and a copy in
// prose survives a toolchain bump silently — the module file is the only place the
// requirement is enforced, so it is the only place it should be written.
func renderGoVersion() string {
	content, err := os.ReadFile("go.mod")
	if err != nil {
		panic(fmt.Errorf("failed to read go.mod: %w", err))
	}

	match := goDirective.FindSubmatch(content)
	if match == nil {
		panic(fmt.Errorf("go.mod has no `go` directive"))
	}

	return string(match[1])
}

// filterExpressionExamples are the worked examples for the `filter-exp` routine
// field, each paired with the client call that produces it.
//
// A filter expression is a base64-encoded binary blob: unreadable, and impossible
// to proofread. A single wrong character is a configuration the service refuses to
// start on, so the encoded value is built here by the same client library a reader
// would use rather than transcribed into the table.
//
//nolint:gochecknoglobals // a table, read by renderFilterExpressions.
var filterExpressionExamples = []struct {
	description string
	expression  *as.Expression
}{
	{
		description: `age > 25`,
		expression:  as.ExpGreater(as.ExpIntBin("age"), as.ExpIntVal(25)),
	},
	{
		description: `country = "US"`,
		expression:  as.ExpEq(as.ExpStringBin("country"), as.ExpStringVal("US")),
	},
	{
		description: `age >= 18 AND (country = "US" OR country = "CA")`,
		expression: as.ExpAnd(
			as.ExpGreaterEq(as.ExpIntBin("age"), as.ExpIntVal(18)),
			as.ExpOr(
				as.ExpEq(as.ExpStringBin("country"), as.ExpStringVal("US")),
				as.ExpEq(as.ExpStringBin("country"), as.ExpStringVal("CA")),
			),
		),
	},
}

// renderFilterExpressions builds the Markdown table of example filter expressions.
func renderFilterExpressions() string {
	var table strings.Builder

	table.WriteString("| Filter | Base64 value |\n")
	table.WriteString("|--------|--------------|\n")

	for _, example := range filterExpressionExamples {
		encoded, err := example.expression.Base64()
		if err != nil {
			panic(fmt.Errorf("failed to encode filter expression %q: %w", example.description, err))
		}

		_, _ = fmt.Fprintf(&table, "| `%s` | `%s` |\n", example.description, encoded)
	}

	return "\n\n" + table.String()
}
