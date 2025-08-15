package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

var (
	styleTabActive = lipgloss.NewStyle().Bold(true)
	styleTab       = lipgloss.NewStyle()
	styleInfo      = lipgloss.NewStyle().Faint(false)
	styleDivider   = lipgloss.NewStyle().Faint(true)
	styleFooter    = lipgloss.NewStyle().Faint(true)

	stylePlus     = lipgloss.NewStyle().Bold(true)
	styleRunning  = lipgloss.NewStyle()
	styleFull     = lipgloss.NewStyle().Bold(true)
	styleIncr     = lipgloss.NewStyle()
	styleDim      = lipgloss.NewStyle().Faint(true)
	styleExpanded = lipgloss.NewStyle().MarginLeft(4)
	styleCursor   = lipgloss.NewStyle().Reverse(true)
	styleChain    = lipgloss.NewStyle().Faint(true)
)

func renderTabs(names []string, active int, width int) string {
	if width <= 0 {
		return ""
	}

	// Render all tabs & track rune positions
	var sb strings.Builder
	type seg struct{ start, end int }
	segs := make([]seg, len(names))

	runePos := 0
	for i, n := range names {
		var s string
		if i == active {
			s = " [" + styleTabActive.Render(n) + "] "
		} else {
			s = "  " + styleTab.Render(n) + "  "
		}
		segs[i].start = runePos
		runePos += utf8.RuneCountInString(s)
		segs[i].end = runePos
		sb.WriteString(s)
	}

	outRunes := []rune(sb.String())
	total := len(outRunes)

	if total <= width {
		return string(outRunes)
	}

	aStart := segs[active].start
	aEnd := segs[active].end
	aLen := aEnd - aStart
	if aLen >= width {
		// Active tab wider than window → just show leftmost part + ellipsis if cut
		visible := outRunes[aStart : aStart+width-3]
		return string(visible) + "..."
	}

	// Try to center active tab
	lo := aStart - (width-aLen)/2
	if lo < 0 {
		lo = 0
	}
	hi := lo + width
	if hi > total {
		hi = total
		lo = hi - width
	}

	// Ensure active tab fully visible
	if aStart < lo {
		lo = aStart
		hi = lo + width
	}
	if aEnd > hi {
		hi = aEnd
		lo = hi - width
	}

	// Clamp
	if lo < 0 {
		lo = 0
	}
	if hi > total {
		hi = total
	}

	// Add ellipses if clipped
	leftEllipsis := lo > 0
	rightEllipsis := hi < total

	// Reserve space for ellipses
	visibleWidth := width
	if leftEllipsis {
		visibleWidth -= 3
	}
	if rightEllipsis {
		visibleWidth -= 3
	}

	visible := outRunes[lo:hi]
	if len(visible) > visibleWidth {
		visible = visible[:visibleWidth]
	}

	result := ""
	if leftEllipsis {
		result += "..."
	}
	result += string(visible)
	if rightEllipsis {
		result += "..."
	}

	return result
}
func presentBar(pct int) string {
	if pct < 0 {
		return "[──────────────]"
	}
	if pct > 100 {
		pct = 100
	}
	total := 14
	fill := pct * total / 100
	if fill < 0 {
		fill = 0
	}
	if fill > total {
		fill = total
	}
	return "[" + repeat("█", fill) + repeat("─", total-fill) + "]"
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
