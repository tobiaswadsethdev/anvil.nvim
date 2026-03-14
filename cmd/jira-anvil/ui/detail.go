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

// Panel indices for the detail view.
// Without PR: panels 0 (IssueInfo) and 1 (Description)
// With PR:    panels 0 (IssueInfo), 1 (PR Overview), 2 (Description), 3 (PR Files)
const (
	panelIssueInfo    = 0
	panelPROverview   = 1
	panelDescription  = 2 // with PR
	panelPRFiles      = 3 // with PR
	panelDescNoPR     = 1 // without PR
)

// DetailModel shows full issue details with optional PR panels.
type DetailModel struct {
	issue        *api.Issue
	prModel      PRDetailModel
	hasPR        bool
	focusedPanel int // 0-indexed panel number

	// Description/Comments panel
	descTabIndex int // 0=Description, 1=Comments
	descViewport viewport.Model

	width  int
	height int
}

func NewDetailModel(issue *api.Issue, w, h int, hasPR bool) DetailModel {
	m := DetailModel{
		issue: issue,
		hasPR: hasPR,
		width: w,
		height: h,
	}
	// Start focused on description panel (most content)
	if hasPR {
		m.focusedPanel = panelDescription
	} else {
		m.focusedPanel = panelDescNoPR
	}

	_, rightW := columnWidths(w)
	_, rightTopH, _ := panelHeights(h, hasPR)
	vpH := rightTopH - 2 - 2 // borders + title line + tab line
	if vpH < 1 {
		vpH = 1
	}
	m.descViewport = viewport.New(rightW-4, vpH)
	m.descViewport.SetContent(renderDescContent(issue, rightW-4))

	if hasPR {
		m.prModel = NewPRDetailModel(w, h)
	}
	return m
}

// numPanels returns the total number of panels for this detail model.
func (m DetailModel) numPanels() int {
	if m.hasPR {
		return 4
	}
	return 2
}

// descPanelIdx returns the panel index for the description panel.
func (m DetailModel) descPanelIdx() int {
	if m.hasPR {
		return panelDescription
	}
	return panelDescNoPR
}

// prFilesPanelIdx returns the panel index for PR files (only valid when hasPR).
func (m DetailModel) prFilesPanelIdx() int {
	return panelPRFiles
}

func columnWidths(totalW int) (leftW, rightW int) {
	leftW = totalW * 35 / 100
	if leftW < 28 {
		leftW = 28
	}
	rightW = totalW - leftW - 1
	if rightW < 20 {
		rightW = 20
	}
	return
}

// panelHeights returns (leftTopH, rightTopH, bottomH) for the two-column layout.
// bottomH is the height for the bottom-left and bottom-right panels (when hasPR).
func panelHeights(totalH int, hasPR bool) (leftTopH, rightTopH, bottomH int) {
	usable := totalH - 1 // subtract help bar
	if usable < 6 {
		usable = 6
	}
	if !hasPR {
		return usable, usable, 0
	}
	leftTopH = usable * 40 / 100
	if leftTopH < 6 {
		leftTopH = 6
	}
	rightTopH = usable * 50 / 100
	if rightTopH < 6 {
		rightTopH = 6
	}
	bottomH = usable - leftTopH
	if bottomH < 6 {
		bottomH = 6
	}
	return
}

func (m DetailModel) setSize(w, h int) DetailModel {
	m.width = w
	m.height = h

	_, rightW := columnWidths(w)
	_, rightTopH, rightBotH := panelHeights(h, m.hasPR)

	// Viewport for description panel: innerH = panelH - 2(borders) - 2(title+tabbar)
	descVpH := rightTopH - 4
	if !m.hasPR {
		descVpH = rightTopH - 4
	}
	if descVpH < 1 {
		descVpH = 1
	}
	m.descViewport.Width = rightW - 4
	m.descViewport.Height = descVpH
	if m.issue != nil {
		m.descViewport.SetContent(renderDescContent(m.issue, rightW-4))
	}

	if m.hasPR {
		m.prModel = m.prModel.setSize(w, h, rightBotH)
	}
	return m
}

func (m DetailModel) update(msg tea.Msg) (DetailModel, tea.Cmd) {
	var cmd tea.Cmd
	focused := m.focusedPanel

	if focused == m.descPanelIdx() {
		m.descViewport, cmd = m.descViewport.Update(msg)
	} else if m.hasPR && focused == m.prFilesPanelIdx() {
		m.prModel, cmd = m.prModel.updateFilesViewport(msg)
	}
	return m, cmd
}

func (m DetailModel) view() string {
	if m.issue == nil {
		return "Loading..."
	}

	leftW, rightW := columnWidths(m.width)
	leftTopH, rightTopH, bottomH := panelHeights(m.height, m.hasPR)

	// Help bar
	helpBar := helpStyle.Width(m.width).Render(
		"  " + keyStyle.Render("Tab/S-Tab") + " panel  " +
			keyStyle.Render("1-" + fmt.Sprintf("%d", m.numPanels())) + " jump  " +
			keyStyle.Render("[/]") + " tab  " +
			keyStyle.Render("↑/↓") + " scroll  " +
			keyStyle.Render("t") + " transition  " +
			keyStyle.Render("c") + " comment  " +
			keyStyle.Render("a") + " assign  " +
			keyStyle.Render("e") + " edit  " +
			keyStyle.Render("o") + " browser  " +
			keyStyle.Render("q") + " back",
	)

	if !m.hasPR {
		// 2-panel layout
		issuePanel := m.renderIssueInfoPanel(leftW, leftTopH, m.focusedPanel == panelIssueInfo)
		descPanel := m.renderDescriptionPanel(rightW, rightTopH, m.focusedPanel == panelDescNoPR)

		row := lipgloss.JoinHorizontal(lipgloss.Top, issuePanel, " ", descPanel)
		return lipgloss.JoinVertical(lipgloss.Left, row, helpBar)
	}

	// 4-panel layout
	issuePanel := m.renderIssueInfoPanel(leftW, leftTopH, m.focusedPanel == panelIssueInfo)
	prOverPanel := m.prModel.renderOverviewPanel(leftW, bottomH, m.focusedPanel == panelPROverview)

	descPanel := m.renderDescriptionPanel(rightW, rightTopH, m.focusedPanel == panelDescription)
	prFilesPanel := m.prModel.renderFilesPanel(rightW, bottomH, m.focusedPanel == panelPRFiles)

	leftCol := lipgloss.JoinVertical(lipgloss.Left, issuePanel, prOverPanel)
	rightCol := lipgloss.JoinVertical(lipgloss.Left, descPanel, prFilesPanel)
	row := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, " ", rightCol)
	return lipgloss.JoinVertical(lipgloss.Left, row, helpBar)
}

func (m DetailModel) renderIssueInfoPanel(outerW, outerH int, active bool) string {
	issue := m.issue
	innerW := outerW - 4
	if innerW < 1 {
		innerW = 1
	}

	var sb strings.Builder
	sb.WriteString(renderPanelTitle(1, "Issue Info", active) + "\n")
	sb.WriteString(strings.Repeat("─", innerW) + "\n")

	// Issue key + summary as title
	keySummary := titleStyle.Render(issue.Key) + " " +
		lipgloss.NewStyle().Bold(true).Render(TruncateString(issue.Fields.Summary, innerW-len(issue.Key)-2))
	sb.WriteString(TruncateString(lipgloss.NewStyle().Render(keySummary), innerW) + "\n\n")

	writeField(&sb, "Status", ColorStatus(issue.Fields.Status.Name))
	writeField(&sb, "Priority", ColorPriority(issue.Fields.Priority.Name))

	assignee := "Unassigned"
	if issue.Fields.Assignee != nil {
		assignee = TruncateString(issue.Fields.Assignee.DisplayName, innerW-16)
	}
	writeField(&sb, "Assignee", assignee)

	reporter := "—"
	if issue.Fields.Reporter != nil {
		reporter = TruncateString(issue.Fields.Reporter.DisplayName, innerW-16)
	}
	writeField(&sb, "Reporter", reporter)

	if !issue.Fields.Created.IsZero() {
		writeField(&sb, "Created", issue.Fields.Created.Format("2006-01-02 15:04"))
	}
	if !issue.Fields.Updated.IsZero() {
		writeField(&sb, "Updated", issue.Fields.Updated.Format("2006-01-02 15:04"))
	}

	if len(issue.Fields.Labels) > 0 {
		writeField(&sb, "Labels", TruncateString(strings.Join(issue.Fields.Labels, ", "), innerW-16))
	}

	style := panelInactiveStyle
	if active {
		style = panelActiveStyle
	}
	innerH := outerH - 2
	if innerH < 1 {
		innerH = 1
	}
	return style.Width(innerW).Height(innerH).Render(sb.String())
}

func (m DetailModel) renderDescriptionPanel(outerW, outerH int, active bool) string {
	innerW := outerW - 4
	innerH := outerH - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	tabs := []string{"Description", "Comments"}
	title := renderPanelTitle(m.descPanelIdx()+1, "Description", active)
	tabBar := renderPanelTabs(tabs, m.descTabIndex, innerW)
	divider := strings.Repeat("─", innerW)

	vpContent := m.descViewport.View()

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		tabBar,
		divider,
		vpContent,
	)

	style := panelInactiveStyle
	if active {
		style = panelActiveStyle
	}
	return style.Width(innerW).Height(innerH).Render(content)
}

// renderDescContent builds the viewport content for the given tab index.
func renderDescContent(issue *api.Issue, width int) string {
	var sb strings.Builder

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
	return sb.String()
}

// renderCommentsContent builds the viewport content for the comments tab.
func renderCommentsContent(issue *api.Issue, width int) string {
	if issue.Fields.Comment == nil || len(issue.Fields.Comment.Comments) == 0 {
		return lipgloss.NewStyle().Foreground(colorMuted).Render("  (no comments)")
	}

	var sb strings.Builder
	for _, comment := range issue.Fields.Comment.Comments {
		author := "Unknown"
		if comment.Author != nil {
			author = comment.Author.DisplayName
		}
		meta := fmt.Sprintf("%s  ·  %s",
			commentMetaStyle.Render(author),
			commentMetaStyle.Render(formatTime(comment.Created.Time)),
		)
		sb.WriteString("  " + meta + "\n")

		body := strings.TrimSpace(adf.Render(comment.Body))
		if body != "" {
			sb.WriteString(indentText(body, 2))
		}
		sb.WriteString("\n")
		sb.WriteString(strings.Repeat("─", width-4) + "\n")
	}
	return sb.String()
}

// refreshDescViewport updates the viewport content based on current tab.
func (m *DetailModel) refreshDescViewport() {
	_, rightW := columnWidths(m.width)
	innerW := rightW - 4
	if innerW < 1 {
		innerW = 1
	}
	if m.descTabIndex == 0 {
		m.descViewport.SetContent(renderDescContent(m.issue, innerW))
	} else {
		m.descViewport.SetContent(renderCommentsContent(m.issue, innerW))
	}
	m.descViewport.GotoTop()
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
