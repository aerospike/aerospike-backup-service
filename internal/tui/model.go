package tui

import (
	"fmt"
	"strings"
	"time"

	m "github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	api Api

	tabs        []string
	activeTab   int
	clusterLine string

	rows          []row
	cursor        int
	width, height int

	showConfirm *confirm
	errLine     string
	lastRefresh time.Time
}

func NewModel(api Api) tea.Model {
	m := model{api: api, activeTab: 0}
	m.tabs = Keys(api.Config().Routines())
	if len(m.tabs) == 0 {
		m.tabs = []string{"(no routines)"}
	}

	m.clusterLine = presentClusterLine(api.Config(), m.tabs[m.activeTab])
	m.rebuildRows()

	return m
}

// Keys returns a slice of all keys from the given map
func Keys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickRefresh(), nil)
}

const refreshEvery = 5 * time.Second

type tickMsg struct{}

func tickRefresh() tea.Cmd {
	return tea.Tick(refreshEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

type confirm struct {
	title string
	onYes tea.Cmd
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.showConfirm != nil {
		switch k := msg.(type) {
		case tea.KeyMsg:
			switch k.String() {
			case "enter":
				cmd := m.showConfirm.onYes
				m2 := m
				m2.showConfirm = nil
				return m2, cmd
			case "esc":
				m2 := m
				m2.showConfirm = nil
				return m2, nil
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		m2 := m
		m2.refreshData(false)
		return m2, tickRefresh()

	case tea.KeyMsg:
		switch msg.String() {
		case "left":
			if m.activeTab > 0 {
				m.activeTab--
				m.clusterLine = presentClusterLine(m.api.Config(), m.tabs[m.activeTab])
				m.rebuildRows()
			}
			return m, nil
		case "right":
			if m.activeTab < len(m.tabs)-1 {
				m.activeTab++
				m.clusterLine = presentClusterLine(m.api.Config(), m.tabs[m.activeTab])
				m.rebuildRows()
			}
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			return m, m.handleEnter()
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *model) rebuildRows() {
	routine := m.tabs[m.activeTab]
	backups := m.api.Backups(routine)

	for _, b := range backups {
		m.rows = append(m.rows, row{kind: rowRunning, backup: &b})
	}
	m.rows = m.rows[:0]
	runningBackup := m.api.RunningBackup(routine)
	if runningBackup == nil {
		m.rows = append(m.rows, row{kind: rowPlus, label: "+"})
	} else {
		m.rows = append(m.rows, row{kind: rowRunning, label: fmt.Sprintf("running (%d)", runningBackup.PercentageDone)})
	}

	m.rows = append(m.rows, groupRows(backups)...)

	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *model) refreshData(force bool) {
	if !force && time.Since(m.lastRefresh) < 500*time.Millisecond {
		return
	}
	m.lastRefresh = time.Now()
	m.rebuildRows()
}

func (m *model) handleEnter() tea.Cmd {
	if len(m.rows) == 0 {
		return nil
	}
	r := m.rows[m.cursor]
	routine := m.tabs[m.activeTab]

	switch r.kind {
	case rowPlus:
		m.showConfirm = &confirm{
			title: "Start full backup",
			onYes: tea.Batch(func() tea.Msg {
				m.api.StartBackup(routine)
				return nil
			}, tickRefresh(),
			),
		}
		return nil
	case rowRunning:
		m.showConfirm = &confirm{
			title: "Cancel running backup?",
			onYes: tea.Batch(func() tea.Msg {
				m.api.CancelBackup(routine)
				return nil
			}, tickRefresh()),
		}
		return nil

	case rowFull, rowIncr:
		req := buildRestoreRequest(routine, r.backup)
		m.showConfirm = &confirm{
			title: "Start restore?",
			onYes: tea.Batch(func() tea.Msg {
				m.api.StartRestore(req)
				return nil
			}, tickRefresh()),
		}
		return nil
	}
	return nil
}

func (m model) View() string {
	var b strings.Builder
	// Tabs
	b.WriteString(renderTabs(m.tabs, m.activeTab, m.width))
	b.WriteString("\n")

	b.WriteString(styleInfo.Render(m.clusterLine))
	b.WriteString("\n")
	b.WriteString(styleDivider.Render(strings.Repeat("─", max(10, m.width))))
	b.WriteString("\n")

	// List
	for i, r := range m.rows {
		isCursor := i == m.cursor

		switch r.kind {
		case rowPlus:
			line := stylePlus.Render(r.label) + stylePlus.Render("  (Enter)")
			b.WriteString(applyCursor(line, isCursor))
		case rowRunning:
			line := renderRunning(r.backup)
			b.WriteString(applyCursor(line, isCursor))
		case rowFull:
			line := "● " + presentBackupLine(*r.backup)
			if chain := m.chainIndexes(); chain[i] {
				line = styleChain.Render("╎ ") + styleFull.Render(line)
			} else {
				line = "  " + styleFull.Render(line)
			}
			b.WriteString(applyCursor(line, isCursor))
		case rowIncr:
			prefix := m.branchPrefix(i)
			line := prefix + "○ " + presentBackupLine(*r.backup)
			if chain := m.chainIndexes(); chain[i] {
				line = styleChain.Render("╎") + " " + styleIncr.Render(line)
			} else {
				line = "  " + styleIncr.Render(line)
			}
			b.WriteString(applyCursor(line, isCursor))
		}
		b.WriteString("\n")
	}

	// confirm modal
	if m.showConfirm != nil {
		b.WriteString("\n")
		b.WriteString(renderConfirm(m.showConfirm.title, m.width))
	}

	// footer
	b.WriteString(styleDivider.Render(strings.Repeat("─", max(10, m.width))))
	b.WriteString("\n")
	b.WriteString(styleFooter.Render("←/→ routines   ↑/↓ list   Enter action   q/Esc quit  |  ● Full ○ Incr  (manual)/(schedule)"))

	return b.String()
}

func (m model) chainIndexes() map[int]bool {
	out := map[int]bool{}
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return out
	}
	if m.rows[m.cursor].kind != rowIncr {
		return out
	}

	fullIdx := -1
	for i := m.cursor - 1; i >= 0; i-- {
		if m.rows[i].kind == rowFull {
			fullIdx = i
			break
		}
		if m.rows[i].kind == rowPlus || m.rows[i].kind == rowRunning {
			break
		}
	}
	if fullIdx == -1 {
		return out
	}
	for i := fullIdx; i <= m.cursor; i++ {
		if m.rows[i].kind == rowFull || m.rows[i].kind == rowIncr {
			out[i] = true
		}
	}

	return out
}

func (m model) branchPrefix(i int) string {
	if i+1 < len(m.rows) && m.rows[i+1].kind == rowIncr {
		return "├─ "
	}
	return "└─ "
}

func renderRunning(bkp *m.BackupDetails) string {
	return "running (Cancel)"
	//
	//
	//pct, eta := presentProgress(bkp)
	//left := "▶ Running…"
	//if pct >= 0 {
	//	left = fmt.Sprintf("▶ Running… %d%%", pct)
	//}
	//right := presentRunningRight(bkp) // typ/source/ns
	//bar := presentBar(pct)
	//meta := presentRunningMeta(bkp, eta)
	//return styleRunning.Render(left + "  " + right + "\n   " + bar + "  " + meta + "\n   Enter = Cancel")
}

func renderConfirm(title string, width int) string {
	box := lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.DoubleBorder())
	body := title + "\n[Enter] Yes   [Esc] No"
	return lipgloss.Place(width, 6, lipgloss.Center, lipgloss.Center, box.Render(body))
}

func applyCursor(s string, isCursor bool) string {
	if !isCursor {
		return " " + s + " "
	}
	return "[" + styleCursor.Render(s) + "]"
}
