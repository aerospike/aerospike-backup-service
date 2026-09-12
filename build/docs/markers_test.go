package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func engineRenderers() map[string]renderer {
	return map[string]renderer{
		"greeting": func(args string) string { return "hello " + args },
		"empty":    func(string) string { return "" },
	}
}

func TestApplyTags_RendersContentBetweenMarkers(t *testing.T) {
	input := []byte("before <!-- tag greeting world -->stale<!-- /tag --> after\n")

	got := string(applyTags(input, engineRenderers()))

	assert.Equal(t, "before <!-- tag greeting world -->hello world<!-- /tag --> after\n", got)
}

// TestApplyTags_LeavesForeignMarkersAlone covers the table-of-contents tool, whose
// <!-- toc --> markers share the document. The literal "tag" keyword is what tells
// the two apart, so no guessing by name is needed.
func TestApplyTags_LeavesForeignMarkersAlone(t *testing.T) {
	input := []byte("<!-- toc -->\n- a link\n<!-- tocstop -->\n")

	got := string(applyTags(input, engineRenderers()))

	assert.Equal(t, string(input), got)
}

func TestApplyTags_PanicsOnUnknownTag(t *testing.T) {
	input := []byte("<!-- tag nonsense --><!-- /tag -->\n")

	assert.PanicsWithError(t,
		`unknown tag "nonsense": no renderer is registered for it`,
		func() { applyTags(input, engineRenderers()) })
}

// TestApplyTags_PanicsOnMissingClose is the failure the old per-marker regexes
// made easy to miss: a region with no closing marker simply generated nothing.
func TestApplyTags_PanicsOnMissingClose(t *testing.T) {
	input := []byte("<!-- tag greeting world -->\nand nothing closes it\n")

	assert.PanicsWithError(t,
		`tag "greeting" has no closing <!-- /tag -->`,
		func() { applyTags(input, engineRenderers()) })
}

// TestApplyTags_PairsEachRegionSeparately guards the case that motivated the
// closing marker: two tags in one sentence must not merge into one region.
func TestApplyTags_PairsEachRegionSeparately(t *testing.T) {
	input := []byte("a <!-- tag greeting one --><!-- /tag --> b <!-- tag greeting two --><!-- /tag --> c\n")

	got := string(applyTags(input, engineRenderers()))

	assert.Equal(t,
		"a <!-- tag greeting one -->hello one<!-- /tag --> b "+
			"<!-- tag greeting two -->hello two<!-- /tag --> c\n",
		got)
}

func TestApplyTags_HandlesEmptyContent(t *testing.T) {
	input := []byte("<!-- tag empty --><!-- /tag -->\n")

	got := string(applyTags(input, engineRenderers()))

	assert.Equal(t, "<!-- tag empty --><!-- /tag -->\n", got)
}

func TestApplyTags_IsIdempotent(t *testing.T) {
	input := []byte("<!-- tag greeting world --><!-- /tag -->\n")

	once := applyTags(input, engineRenderers())
	twice := applyTags(once, engineRenderers())

	assert.Equal(t, string(once), string(twice))
}

// TestNewRenderers_PanicsOnDuplicateID keeps one namespace honest: if two sources
// claimed the same id, which one a document got would depend on map iteration.
func TestNewRenderers_PanicsOnDuplicateID(t *testing.T) {
	clash := map[string]endpoint{
		"Metrics": {method: "GET", path: "/metrics", tag: "System"},
	}

	assert.Panics(t, func() { newRenderers(clash, "") })
}

// TestNoArgs_PanicsOnUnexpectedArgument closes the gap that only endpoint tags
// read their arguments: without this, <!-- tag Storage nonsense --> would render
// the example and quietly drop the word.
func TestNoArgs_PanicsOnUnexpectedArgument(t *testing.T) {
	render := noArgs("Storage", func() string { return "content" })

	assert.Equal(t, "content", render(""))
	assert.PanicsWithError(t,
		`tag "Storage" takes no arguments, got "nonsense"`,
		func() { render("nonsense") })
}

func TestRenderExample_PanicsOnUnknownExample(t *testing.T) {
	assert.Panics(t, func() { renderExample("NoSuchExample") })
}

// TestApplyTags_LeavesTagsInsideFencesAlone is what lets docs/development.md
// document the tag syntax: inside a fenced block a tag is being shown, not used,
// so expanding it would rewrite the explanation into an example of itself.
func TestApplyTags_LeavesTagsInsideFencesAlone(t *testing.T) {
	input := []byte("```markdown\n<!-- tag greeting world --><!-- /tag -->\n```\n")

	got := string(applyTags(input, engineRenderers()))

	assert.Equal(t, string(input), got)
}

// TestApplyTags_RendersAroundAFencedExample covers the two together: a fenced
// illustration must not stop the real tags on either side of it from rendering.
func TestApplyTags_RendersAroundAFencedExample(t *testing.T) {
	input := []byte("<!-- tag greeting one --><!-- /tag -->\n" +
		"```markdown\n<!-- tag nonsense --><!-- /tag -->\n```\n" +
		"<!-- tag greeting two --><!-- /tag -->\n")

	got := string(applyTags(input, engineRenderers()))

	assert.Equal(t, "<!-- tag greeting one -->hello one<!-- /tag -->\n"+
		"```markdown\n<!-- tag nonsense --><!-- /tag -->\n```\n"+
		"<!-- tag greeting two -->hello two<!-- /tag -->\n", got)
}

// TestApplyTags_IgnoresAnUnclosedTagInsideAFence keeps the fence rule and the
// missing-close rule from contradicting each other.
func TestApplyTags_IgnoresAnUnclosedTagInsideAFence(t *testing.T) {
	input := []byte("```markdown\n<!-- tag greeting world -->\n```\n")

	assert.NotPanics(t, func() { applyTags(input, engineRenderers()) })
}

// TestApplyTags_SeesTagsAfterAFencedBlock guards the fence counter itself: a
// generated region carries its own fences, and if they were counted as unpaired
// every tag below the first rendered example would be treated as illustration.
func TestApplyTags_SeesTagsAfterAFencedBlock(t *testing.T) {
	input := []byte("```yaml\nservice: {}\n```\n<!-- tag greeting world --><!-- /tag -->\n")

	got := string(applyTags(input, engineRenderers()))

	assert.Contains(t, got, "<!-- tag greeting world -->hello world<!-- /tag -->")
}
