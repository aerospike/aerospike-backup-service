package tui

import (
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
