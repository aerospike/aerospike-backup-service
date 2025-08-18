package tui

import (
	"strings"
	"unicode/utf8"
)

// RenderTabs builds three lines of tabs and returns them joined by '\n'.
// - tabs: labels
// - selected: index of the active tab (0-based; clamped into range)
// - width: viewport width in monospace cells
// RenderTabs returns three lines joined by '\n'.
// - tabs: labels
// - selected: active tab index
// - width: viewport width
func renderTabs(tabs []string, selected, width int) string {
	if width <= 0 || len(tabs) == 0 {
		return ""
	}

	const pad = 1 // space padding on each side of label

	// Layout: compute per-tab [start,end) across a single long row
	type tabPos struct{ start, end int }
	pos := make([]tabPos, len(tabs))
	x := 0
	for i, t := range tabs {
		w := runeWidth(t) + pad*2
		pos[i] = tabPos{start: x, end: x + w}
		x += w
		if i != len(tabs)-1 {
			x += 1 // divider column
		}
	}
	total := x

	// Clamp selected
	if selected < 0 {
		selected = 0
	}
	if selected >= len(tabs) {
		selected = len(tabs) - 1
	}

	// Center the active tab when possible
	tabStart := pos[selected].start
	tabEnd := pos[selected].end
	tabCenter := (tabStart + tabEnd) / 2

	var start int
	if total <= width {
		start = 0
	} else {
		start = tabCenter - width/2
		start = max(0, min(start, total-width))
	}

	winStart := start
	winEnd := start + width

	// Buffers
	top := make([]rune, width)
	mid := make([]rune, width)
	bot := make([]rune, width)
	for i := 0; i < width; i++ {
		top[i] = ' '
		mid[i] = ' '
		bot[i] = '─' // baseline default
	}

	// Draw
	for i, t := range tabs {
		tStart := pos[i].start
		tEnd := pos[i].end

		// divider column (between tabs) -> draw a vertical in mid and a ┴ on bottom
		if i != len(tabs)-1 {
			divAbs := tEnd
			if divAbs >= winStart && divAbs < winEnd {
				col := divAbs - winStart
				// Don't overwrite the active gap later; we’ll clear baseline under active after
				mid[col] = ' '
				bot[col] = '─'
			}
		}

		// Skip if off-screen
		if tEnd <= winStart || tStart >= winEnd {
			continue
		}

		if i == selected {
			// ACTIVE: top box
			if tStart >= winStart && tStart < winEnd {
				top[tStart-winStart] = '┌'
			}
			if tEnd-1 >= winStart && tEnd-1 < winEnd {
				top[tEnd-1-winStart] = '┐'
			}
			fillRunes(top, max(tStart+1, winStart)-winStart, min(tEnd-1, winEnd)-winStart, '─')

			// Middle: vertical sides + label
			if tStart >= winStart && tStart < winEnd {
				mid[tStart-winStart] = '│'
			}
			if tEnd-1 >= winStart && tEnd-1 < winEnd {
				mid[tEnd-1-winStart] = '│'
			}
			writeLabel(mid, width, max(tStart+pad, winStart)-winStart, min(tEnd-pad, winEnd)-winStart, t)

			// Bottom: open a gap, and add neat tees at the gap edges
			fillRunes(bot, max(tStart, winStart)-winStart, min(tEnd, winEnd)-winStart, ' ')
			if tStart >= winStart && tStart < winEnd {
				bot[tStart-winStart] = '┘'
			}
			if tEnd-1 >= winStart && tEnd-1 < winEnd {
				bot[tEnd-1-winStart] = '└'
			}
		} else {
			// INACTIVE: no top edge; just label on middle line
			writeLabel(mid, width, max(tStart+pad, winStart)-winStart, min(tEnd-pad, winEnd)-winStart, t)
		}
	}

	// Truncation marks ("...") if cropped left/right — placed on the middle line.
	putEllipsis := func(left bool) {
		if width <= 0 {
			return
		}
		if left {
			// place at column 0..2
			for i, ch := range []rune{'.', '.', '.'} {
				if i < width {
					mid[i] = ch
				}
			}
		} else {
			// place ending at width-1
			if width == 1 {
				mid[0] = '.'
				return
			}
			if width == 2 {
				mid[width-2], mid[width-1] = '.', '.'
				return
			}
			mid[width-3], mid[width-2], mid[width-1] = '.', '.', '.'
		}
	}
	if winStart > 0 {
		putEllipsis(true)
	}
	if winEnd < total {
		putEllipsis(false)
	}

	return string(top) + "\n" + string(mid) + "\n" + string(bot)
}

// Helpers

func fillRunes(buf []rune, start, end int, r rune) {
	start = max(0, start)
	end = min(end, len(buf))
	for i := start; i < end; i++ {
		buf[i] = r
	}
}

// Center label and truncate with ellipsis if needed.
func writeLabel(buf []rune, width, start, end int, label string) {
	if start >= end || end <= 0 || start >= width {
		return
	}
	start = max(0, start)
	end = min(end, width)
	slot := end - start
	if slot <= 0 {
		return
	}

	lw := runeWidth(label)
	text := label
	if lw > slot {
		// reserve 3 for "..."
		if slot <= 3 {
			text = strings.Repeat(".", slot) // all dots if too tight
		} else {
			text = truncateToWidth(label, slot-3) + "..."
		}
		lw = runeWidth(text)
	}

	leftPad := (slot - lw) / 2
	idx := start + leftPad
	for _, r := range text {
		if idx >= end {
			break
		}
		buf[idx] = r
		idx++
	}
}

// Monospace rune width (ok for ASCII + box-drawing)
func runeWidth(s string) int { return utf8.RuneCountInString(s) }

func truncateToWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	n := 0
	var b strings.Builder
	for _, r := range s {
		if n >= w {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}
