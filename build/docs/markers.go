package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// A tag is a span of generated content in an otherwise hand-written document:
//
//	<!-- tag <id> [args] --> … generated … <!-- /tag -->
//
// One form, one engine. Before this there was a regex per kind of generated
// content, each with its own idea of how far the generated text reached — a
// fenced block, a table, a link, the rest of the line — and each one a chance to
// match too much or too little. The closing marker answers that question once,
// so adding a kind of generated content means registering a renderer.
//
// The literal "tag" keyword is what separates this project's markers from the
// ones other tools leave in the same documents, such as the <!-- toc --> written
// by the table-of-contents generator. Anything that is not a tag is left alone.
//
// A renderer returns only the content. The engine re-emits the markers around it,
// so a region always survives to be regenerated next time.
type renderer func(args string) string

// openTagPattern matches an opening marker, capturing the id and the arguments
// after it. Both regexes below are built from it so the two cannot drift: the
// region matcher and the unclosed-region check must agree on what an opening
// marker looks like, or a forgotten <!-- /tag --> stops being an error.
const openTagPattern = `<!--\s*tag\s+(\S+)[ \t]*(.*?)-->`

// tagRegion matches one complete region. The content is non-greedy, so each
// opening marker pairs with the nearest closing one and two tags on the same line
// stay separate.
var tagRegion = regexp.MustCompile(`(?s)` + openTagPattern + `(?:.*?)<!--\s*/tag\s*-->`)

// openTag matches any opening tag, to find regions that lost their close.
var openTag = regexp.MustCompile(openTagPattern)

// applyTags renders every tagged region in the document. An unknown id is a typo
// and stops the build rather than silently generating nothing.
//
// A tag written inside a fenced code block is left exactly as it is: there it is
// being shown, not used, which is how docs/development.md can document the syntax
// without the generator expanding the example out from under it.
func applyTags(content []byte, renderers map[string]renderer) []byte {
	fenced := fencedOffsets(content)

	var (
		rendered []byte
		last     int
	)

	for _, region := range tagRegion.FindAllSubmatchIndex(content, -1) {
		start, end := region[0], region[1]
		if fenced[start] {
			continue
		}

		id := string(content[region[2]:region[3]])
		args := strings.TrimSpace(string(content[region[4]:region[5]]))

		render, known := renderers[id]
		if !known {
			panic(fmt.Errorf("unknown tag %q: no renderer is registered for it", id))
		}

		rendered = append(rendered, content[last:start]...)
		rendered = fmt.Appendf(rendered, "<!-- tag %s -->%s<!-- /tag -->",
			strings.TrimSpace(id+" "+args), render(args))
		last = end
	}

	rendered = append(rendered, content[last:]...)

	requireClosed(rendered)

	return rendered
}

// fenceLine matches the start of a Markdown code fence.
var fenceLine = regexp.MustCompile("(?m)^[ \t]*(```|~~~)")

// fencedOffsets reports, for every byte offset, whether it sits inside a fenced
// code block.
//
// Generated content always carries balanced fences — a rendered example opens and
// closes its own — so counting fences across the whole document, generated regions
// included, stays in step. Only a document that shows an unpaired fence would
// confuse this, and such a document does not render correctly in the first place.
func fencedOffsets(content []byte) []bool {
	inside := make([]bool, len(content)+1)

	var (
		offset int
		open   bool
	)

	for _, line := range bytes.SplitAfter(content, []byte("\n")) {
		isFence := fenceLine.Match(line)
		if isFence {
			open = !open
		}

		// A fence line belongs to the block it delimits, not to the side it
		// switches to: after the toggle, the opening line reads as outside and
		// the closing line as inside. Neither can carry a tag, so this only has
		// to be consistent.
		within := open != isFence
		for i := range line {
			inside[offset+i] = within
		}

		offset += len(line)
	}

	inside[len(content)] = open

	return inside
}

// requireClosed reports an opening tag that no closing marker follows. Without it
// a forgotten <!-- /tag --> would simply generate nothing, which is the failure
// the old per-marker regexes made easy to miss.
func requireClosed(content []byte) {
	fenced := fencedOffsets(content)

	closed := make(map[int]bool)
	for _, region := range tagRegion.FindAllIndex(content, -1) {
		closed[region[0]] = true
	}

	for _, opening := range openTag.FindAllSubmatchIndex(content, -1) {
		if closed[opening[0]] || fenced[opening[0]] {
			continue
		}

		id := string(content[opening[2]:opening[3]])
		panic(fmt.Errorf("tag %q has no closing <!-- /tag -->", id))
	}
}

// renderExample renders one of the worked examples built from the DTO structs.
// The format follows from which collection defines the example, so a document
// only has to name it.
func renderExample(name string) string {
	if example, defined := jsonExamples[name]; defined {
		content, err := json.MarshalIndent(example, "", "  ")
		if err != nil {
			panic(fmt.Errorf("marshal example %q: %w", name, err))
		}

		return fence("json", content)
	}

	example, defined := yamlExamples[name]
	if !defined {
		panic(fmt.Errorf("unknown example %q: it is in neither jsonExamples nor yamlExamples", name))
	}

	content, err := marshalYAML(example)
	if err != nil {
		panic(fmt.Errorf("marshal example %q: %w", name, err))
	}

	return fence("yaml", content)
}

// fence wraps generated content in a Markdown code fence, spaced so the rendered
// document reads the same as when the blocks were written by hand.
func fence(language string, content []byte) string {
	return "\n\n```" + language + "\n" + string(content) + "\n```\n"
}

// noArgs adapts a renderer that takes no arguments.
//
// Only endpoint tags read their arguments, so without this an id that ignores
// them would silently accept anything: <!-- tag Storage nonsense --> would
// render the storage example and quietly drop the word. A typo should stop the
// build for the same reason an unknown id does.
func noArgs(id string, render func() string) renderer {
	return func(args string) string {
		if args != "" {
			panic(fmt.Errorf("tag %q takes no arguments, got %q", id, args))
		}

		return render()
	}
}
