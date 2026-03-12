package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/api"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/ui/adf"
)

// DetailModel shows full issue details with optional PR tab.
type DetailModel struct {
	issue    *api.Issue
	viewport viewport.Model
	prModel  PRDetailModel
	tabIndex int  // 0 = Jira, 1 = Pull Request
	hasPRTab bool // true when Azure DevOps is configured
	width    int
	height   int
}

func NewDetailModel(issue *api.Issue, w, h int, hasPRTab bool) DetailModel {
	vpHeight := jiraViewportHeight(h, hasPRTab)
	vp := viewport.New(w, vpHeight)
	vp.SetContent(renderIssueContent(issue, w))
	m := DetailModel{
		issue:    issue,
		viewport: vp,
		hasPRTab: hasPRTab,
		width:    w,
		height:   h,
	}
	if hasPRTab {
		m.prModel = NewPRDetailModel(w, h)
	}
	return m
}

func jiraViewportHeight(h int, hasPRTab bool) int {
	if hasPRTab {
		// title(1) + tabBar(1) + statusBar(1) + helpBar(1) + padding(3)
		return h - 7
	}
	return h - 6
}

func (m DetailModel) setSize(w, h int) DetailModel {
	m.width = w
	m.height = h
	m.viewport.Width = w
	m.viewport.Height = jiraViewportHeight(h, m.hasPRTab)
	if m.issue != nil {
		m.viewport.SetContent(renderIssueContent(m.issue, w))
	}
	if m.hasPRTab {
		m.prModel = m.prModel.setSize(w, h)
	}
	return m
}

func (m DetailModel) update(msg tea.Msg) (DetailModel, tea.Cmd) {
	var cmd tea.Cmd
	if m.tabIndex == 1 && m.hasPRTab {
		m.prModel, cmd = m.prModel.update(msg)
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
	}
	return m, cmd
}

func (m DetailModel) view() string {
	if m.issue == nil {
		return "Loading..."
	}

	// Title bar
	titleLine := titleStyle.Render(m.issue.Key) + " " +
		lipgloss.NewStyle().Bold(true).Render(m.issue.Fields.Summary)
	titleBar := lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color("237")).
		Padding(0, 1).
		Render(TruncateString(titleLine, m.width-4))

	// Tab bar (shown when Azure DevOps is configured)
	var tabBar string
	if m.hasPRTab {
		tabBar = renderTabBar(m.tabIndex, m.width)
	}

	// Content viewport
	var content string
	if m.tabIndex == 1 && m.hasPRTab {
		content = m.prModel.view()
	} else {
		content = m.viewport.View()
	}

	// Scroll percentage for active tab
	var scrollPct int
	if m.tabIndex == 1 && m.hasPRTab {
		scrollPct = int(m.prModel.viewport.ScrollPercent() * 100)
	} else {
		scrollPct = int(m.viewport.ScrollPercent() * 100)
	}
	scrollInfo := fmt.Sprintf("%d%%", scrollPct)

	// Status bar
	statusBar := statusBarStyle.Width(m.width).Render(
		fmt.Sprintf(" %s  %s  %s  %s",
			ColorStatus(m.issue.Fields.Status.Name),
			"•",
			ColorPriority(m.issue.Fields.Priority.Name),
			lipgloss.NewStyle().Foreground(colorMuted).Render(scrollInfo),
		),
	)

	// Help bar
	tabHint := ""
	if m.hasPRTab {
		tabHint = keyStyle.Render("[/]") + " tab  "
	}
	helpBar := helpStyle.Width(m.width).Render(
		"  " + tabHint +
			keyStyle.Render("↑/↓") + " scroll  " +
			keyStyle.Render("t") + " transition  " +
			keyStyle.Render("c") + " comment  " +
			keyStyle.Render("a") + " assign  " +
			keyStyle.Render("e") + " edit  " +
			keyStyle.Render("v") + " vote  " +
			keyStyle.Render("o") + " browser  " +
			keyStyle.Render("q") + " back",
	)

	parts := []string{titleBar}
	if m.hasPRTab {
		parts = append(parts, tabBar)
	}
	parts = append(parts, content, statusBar, helpBar)
	return strings.Join(parts, "\n")
}

// renderTabBar renders the tab navigation bar.
func renderTabBar(activeTab, width int) string {
	tabs := []string{"Jira", "Pull Request"}

	activeTabStyle := lipgloss.NewStyle().
		Background(colorSelected).
		Foreground(colorFg).
		Bold(true).
		Padding(0, 2)

	inactiveTabStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("237")).
		Foreground(colorMuted).
		Padding(0, 2)

	var parts []string
	for i, tab := range tabs {
		if i == activeTab {
			parts = append(parts, activeTabStyle.Render(tab))
		} else {
			parts = append(parts, inactiveTabStyle.Render(tab))
		}
	}

	bar := strings.Join(parts, "")
	barWidth := lipgloss.Width(bar)
	if width > barWidth {
		filler := lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Render(strings.Repeat(" ", width-barWidth))
		bar += filler
	}
	return bar
}

func renderIssueContent(issue *api.Issue, width int) string {
	var sb strings.Builder

	// Fields header
	sb.WriteString(sectionStyle.Render("Details") + "\n")
	writeField(&sb, "Status", ColorStatus(issue.Fields.Status.Name))
	writeField(&sb, "Priority", ColorPriority(issue.Fields.Priority.Name))

	assignee := "Unassigned"
	if issue.Fields.Assignee != nil {
		assignee = issue.Fields.Assignee.DisplayName
	}
	writeField(&sb, "Assignee", assignee)

	reporter := "—"
	if issue.Fields.Reporter != nil {
		reporter = issue.Fields.Reporter.DisplayName
	}
	writeField(&sb, "Reporter", reporter)

	if !issue.Fields.Created.IsZero() {
		writeField(&sb, "Created", issue.Fields.Created.Format("2006-01-02 15:04"))
	}
	if !issue.Fields.Updated.IsZero() {
		writeField(&sb, "Updated", issue.Fields.Updated.Format("2006-01-02 15:04"))
	}

	if len(issue.Fields.Labels) > 0 {
		writeField(&sb, "Labels", strings.Join(issue.Fields.Labels, ", "))
	}

	// Custom ADF fields
	if len(issue.Fields.Custom) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Custom Fields") + "\n")
		for name, raw := range issue.Fields.Custom {
			text := strings.TrimSpace(adf.Render(raw))
			if text != "" {
				sb.WriteString("\n")
				sb.WriteString(fieldLabelStyle.Render(name+":\n"))
				sb.WriteString(indentText(text, 2))
				sb.WriteString("\n")
			}
		}
	}

	// Description
	sb.WriteString("\n")
	sb.WriteString(sectionStyle.Render("Description") + "\n\n")
	if issue.Fields.Description != nil {
		desc := strings.TrimSpace(adf.Render(issue.Fields.Description))
		if desc != "" {
			sb.WriteString(indentText(desc, 2))
		} else {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render("  (no description)"))
		}
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render("  (no description)"))
	}
	sb.WriteString("\n")

	// Comments
	if issue.Fields.Comment != nil && len(issue.Fields.Comment.Comments) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render(
			fmt.Sprintf("Comments (%d)", issue.Fields.Comment.Total),
		) + "\n")

		for _, comment := range issue.Fields.Comment.Comments {
			sb.WriteString("\n")
			author := "Unknown"
			if comment.Author != nil {
				author = comment.Author.DisplayName
			}
			meta := fmt.Sprintf("%s  ·  %s",
				commentMetaStyle.Render(author),
				commentMetaStyle.Render(formatTime(comment.Created)),
			)
			sb.WriteString("  " + meta + "\n")

			body := strings.TrimSpace(adf.Render(comment.Body))
			if body != "" {
				sb.WriteString(indentText(body, 2))
			}
			sb.WriteString("\n")
			sb.WriteString(strings.Repeat("─", width-4) + "\n")
		}
	}

	return sb.String()
}

func writeField(sb *strings.Builder, label, value string) {
	sb.WriteString(fieldLabelStyle.Render(label+":") + " " + fieldValueStyle.Render(value) + "\n")
}

func indentText(text string, spaces int) string {
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(text, "\n")
	var result []string
	for _, line := range lines {
		result = append(result, indent+line)
	}
	return strings.Join(result, "\n") + "\n"
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}
