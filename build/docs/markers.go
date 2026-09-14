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

// openTag matches an opening marker, capturing the id and the arguments after
// it. closeTag matches the marker that ends a region.
var (
	openTag  = regexp.MustCompile(`<!--\s*tag\s+(\S+)[ \t]*(.*?)-->`)
	closeTag = regexp.MustCompile(`<!--\s*/tag\s*-->`)
)

// applyTags renders every tagged region in the document.
//
// Regions are found by walking the opening markers in order and pairing each
// with the nearest closing marker after it. That pairing is only accepted when
// no other opening marker lies in between: a tag whose own close is missing or
// mistyped would otherwise borrow the next tag's, and rendering would replace
// every line of prose between the two — silently, since the borrowed region
// looks perfectly well-formed. Either mistake stops the build instead, which is
// what docs/development.md promises. An unknown id is a typo and stops it too.
//
// A tag written inside a fenced code block is left exactly as it is: there it is
// being shown, not used, which is how docs/development.md can document the syntax
// without the generator expanding the example out from under it.
func applyTags(content []byte, renderers map[string]renderer) []byte {
	fenced := fencedOffsets(content)
	openers := openTag.FindAllSubmatchIndex(content, -1)

	var (
		rendered []byte
		last     int
	)

	for i, opening := range openers {
		start := opening[0]
		if fenced[start] {
			continue
		}

		id := string(content[opening[2]:opening[3]])
		args := strings.TrimSpace(string(content[opening[4]:opening[5]]))

		closing := closeTag.FindIndex(content[opening[1]:])
		if closing == nil {
			panic(fmt.Errorf("tag %q has no closing <!-- /tag -->", id))
		}

		closeStart, closeEnd := opening[1]+closing[0], opening[1]+closing[1]

		if i+1 < len(openers) && openers[i+1][0] < closeStart {
			next := string(content[openers[i+1][2]:openers[i+1][3]])
			panic(fmt.Errorf("tag %q is not closed before tag %q opens; "+
				"a <!-- /tag --> is missing or mistyped", id, next))
		}

		render, known := renderers[id]
		if !known {
			panic(fmt.Errorf("unknown tag %q: no renderer is registered for it", id))
		}

		rendered = append(rendered, content[last:start]...)
		rendered = fmt.Appendf(rendered, "<!-- tag %s -->%s<!-- /tag -->",
			strings.TrimSpace(id+" "+args), render(args))
		last = closeEnd
	}

	return append(rendered, content[last:]...)
}

// fenceLine matches the start of a Markdown code fence.
var fenceLine = regexp.MustCompile("(?m)^[ \t]*(```|~~~)")

// fencedOffsets reports, for every byte offset, whether it sits inside a fenced
// code block.
//
// Generated content always carries balanced fences — a rendered example opens and
// closes its own — so counting fences across the whole document, generated regions
// included, stays in step. A fence that is still open at the end of the document
// is a build failure: every tag after it would otherwise be treated as
// illustration and quietly left stale, and such a document does not render
// correctly in the first place.
func fencedOffsets(content []byte) []bool {
	inside := make([]bool, len(content)+1)

	var (
		offset   int
		open     bool
		openedAt int // 1-based line of the fence that is currently open
		line     int
	)

	for _, text := range bytes.SplitAfter(content, []byte("\n")) {
		line++

		isFence := fenceLine.Match(text)
		if isFence {
			open = !open
			if open {
				openedAt = line
			}
		}

		// A fence line belongs to the block it delimits, not to the side it
		// switches to: after the toggle, the opening line reads as outside and
		// the closing line as inside. Neither can carry a tag, so this only has
		// to be consistent.
		within := open != isFence
		for i := range text {
			inside[offset+i] = within
		}

		offset += len(text)
	}

	if open {
		panic(fmt.Errorf("the code fence opened on line %d is never closed; "+
			"every tag after it would be ignored", openedAt))
	}

	inside[len(content)] = false

	return inside
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
	return "\n\n```" + language + "\n" + string(bytes.TrimRight(content, "\n")) + "\n```\n"
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
