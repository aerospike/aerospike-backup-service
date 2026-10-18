package doccheck

import (
	"regexp"
	"strings"
)

// CodeBlock is a fenced code block in a Markdown document.
type CodeBlock struct {
	// Language is the first word of the info string, e.g. "yaml".
	Language string
	// Args is the rest of the info string, e.g. "dto=Config".
	Args string
	// Body is the content between the fences.
	Body string
	// Line is the 1-based line of the opening fence.
	Line int
	// Generated reports whether the block sits inside a <!-- tag --> region, so
	// build/docs rewrites it from the code on every run.
	Generated bool
}

var (
	fenceLine = regexp.MustCompile("^[ \t]*(```|~~~)(.*)$")
	tagMarker = regexp.MustCompile(`<!--\s*(/?)tag\b`)
)

// CodeBlocks returns every fenced code block in a Markdown document.
//
// Tag markers are followed only outside fences: inside one they are being shown,
// not used, which is also how build/docs reads them.
func CodeBlocks(content string) []CodeBlock {
	var (
		blocks  []CodeBlock
		current *CodeBlock
		body    []string
		inTag   bool
	)

	for i, line := range strings.Split(content, "\n") {
		if fence := fenceLine.FindStringSubmatch(line); fence != nil {
			if current == nil {
				language, args, _ := strings.Cut(strings.TrimSpace(fence[2]), " ")
				current = &CodeBlock{
					Language:  language,
					Args:      strings.TrimSpace(args),
					Line:      i + 1,
					Generated: inTag,
				}
				body = nil

				continue
			}

			current.Body = strings.Join(body, "\n")
			blocks = append(blocks, *current)
			current = nil

			continue
		}

		if current != nil {
			body = append(body, line)
			continue
		}

		for _, marker := range tagMarker.FindAllStringSubmatch(line, -1) {
			inTag = marker[1] == ""
		}
	}

	return blocks
}
