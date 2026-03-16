package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/api"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/config"
)

// ListModel is the issue list view.
type ListModel struct {
	cfg         *config.Config
	client      *api.Client
	filterIndex int
	issues      []api.Issue
	total       int
	loading     bool
	spinner     spinner.Model
	table       table.Model
	width       int
	height      int
}

type listRects struct {
	filter  Rect
	content Rect
	status  Rect
	help    Rect
}

func computeListRects(w, h int) listRects {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	filterH, statusH, helpH := 1, 1, 1
	contentH := h - filterH - statusH - helpH
	if contentH < 1 {
		contentH = 1
	}

	return listRects{
		filter:  Rect{X: 0, Y: 0, W: w, H: filterH},
		content: Rect{X: 0, Y: filterH, W: w, H: contentH},
		status:  Rect{X: 0, Y: filterH + contentH, W: w, H: statusH},
		help:    Rect{X: 0, Y: filterH + contentH + statusH, W: w, H: helpH},
	}
}

func NewListModel(cfg *config.Config, client *api.Client, sp spinner.Model) ListModel {
	t := table.New(
		table.WithFocused(true),
		table.WithStyles(tableStyles()),
	)
	return ListModel{
		cfg:     cfg,
		client:  client,
		loading: true,
		spinner: sp,
		table:   t,
	}
}

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true).
		Foreground(colorSecond)
	s.Selected = s.Selected.
		Foreground(colorFg).
		Background(colorSelected).
		Bold(false)
	return s
}

func (m ListModel) setSize(w, h int) ListModel {
	m.width = w
	m.height = h
	m.buildTable()
	return m
}

func (m *ListModel) buildTable() {
	if m.width == 0 {
		return
	}

	// Calculate column widths dynamically
	keyW := 12
	statusW := 16
	priorityW := 12
	assigneeW := 18
	updatedW := 12
	summaryW := m.width - keyW - statusW - priorityW - assigneeW - updatedW - 10

	if summaryW < 20 {
		summaryW = 20
	}

	cols := []table.Column{
		{Title: "KEY", Width: keyW},
		{Title: "SUMMARY", Width: summaryW},
		{Title: "STATUS", Width: statusW},
		{Title: "PRIORITY", Width: priorityW},
		{Title: "ASSIGNEE", Width: assigneeW},
		{Title: "UPDATED", Width: updatedW},
	}

	rows := make([]table.Row, 0, len(m.issues))
	for _, issue := range m.issues {
		assignee := "—"
		if issue.Fields.Assignee != nil {
			assignee = TruncateString(issue.Fields.Assignee.DisplayName, assigneeW-1)
		}
		rows = append(rows, table.Row{
			issue.Key,
			TruncateString(issue.Fields.Summary, summaryW-1),
			issue.Fields.Status.Name,
			issue.Fields.Priority.Name,
			assignee,
			formatAge(issue.Fields.Updated.Time),
		})
	}

	rects := computeListRects(m.width, m.height)
	tableH := rects.content.H - 1 // table header consumes one row
	if tableH < 3 {
		tableH = 3
	}

	m.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(tableH),
		table.WithStyles(tableStyles()),
	)
}

func (m ListModel) selectedIssue() *api.Issue {
	if len(m.issues) == 0 {
		return nil
	}
	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= len(m.issues) {
		return nil
	}
	return &m.issues[cursor]
}

func (m ListModel) fetchCmd() tea.Cmd {
	filter := m.cfg.Filters[m.filterIndex]
	return func() tea.Msg {
		issues, total, err := m.client.SearchIssues(filter.JQL, 100)
		if err != nil {
			return errMsg{err}
		}
		return issuesFetchedMsg{issues, total}
	}
}

func (m ListModel) update(msg tea.Msg) (ListModel, tea.Cmd) {
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m ListModel) view() string {
	filter := m.cfg.Filters[m.filterIndex]
	filterCount := len(m.cfg.Filters)
	rects := computeListRects(m.width, m.height)

	// Filter bar
	nav := ""
	if filterCount > 1 {
		nav = fmt.Sprintf(" [%d/%d]", m.filterIndex+1, filterCount)
	}
	filterBar := filterBarStyle.Render(
		fmt.Sprintf(" ◀[  %s%s  ]▶", filter.Name, nav),
	)
	filterBar = lipgloss.NewStyle().Width(rects.filter.W).Render(filterBar)

	// Content
	var content string
	if m.loading {
		content = lipgloss.NewStyle().
			Width(rects.content.W).
			Height(rects.content.H).
			Align(lipgloss.Center, lipgloss.Center).
			Render(m.spinner.View() + " Loading...")
	} else if len(m.issues) == 0 {
		content = lipgloss.NewStyle().
			Width(rects.content.W).
			Height(rects.content.H).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(colorMuted).
			Render("No issues found for this filter.")
	} else {
		content = m.table.View()
	}

	// Status bar
	statusText := fmt.Sprintf(" %d issues • %s", m.total, filter.JQL)
	if len(statusText) > rects.status.W-2 {
		statusText = TruncateString(statusText, rects.status.W-2)
	}
	statusBar := statusBarStyle.Width(rects.status.W).Render(statusText)

	// Help bar
	helpBar := helpStyle.Width(rects.help.W).Render(
		"  " + keyStyle.Render("[/]") + " cycle  " +
			keyStyle.Render("Enter") + " open  " +
			keyStyle.Render("r") + " refresh  " +
			keyStyle.Render("n") + " new  " +
			keyStyle.Render("o") + " browser  " +
			keyStyle.Render("q") + " quit",
	)

	return composeRectGrid(
		rects.filter.W,
		rects.help.Y+rects.help.H,
		[]positionedBlock{
			{rect: rects.filter, lines: normalizeBlock(filterBar, rects.filter.W, rects.filter.H)},
			{rect: rects.content, lines: normalizeBlock(content, rects.content.W, rects.content.H)},
			{rect: rects.status, lines: normalizeBlock(statusBar, rects.status.W, rects.status.H)},
			{rect: rects.help, lines: normalizeBlock(helpBar, rects.help.W, rects.help.H)},
		},
	)
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	default:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	}
}
