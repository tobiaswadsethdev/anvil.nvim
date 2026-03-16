package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/api"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/ui/adf"
)

const (
	panelIssueInfo = 0
	panelPRInfo    = 1
	panelCenter    = 2
	panelRight     = 3
	panelDescNoPR  = 1
)

type DetailModel struct {
	issue          *api.Issue
	prModel        PRDetailModel
	hasPR          bool
	focusedPanel   int
	issueViewport  viewport.Model
	prInfoViewport viewport.Model

	// No-PR mode (2 panels)
	descTabIndex int
	descViewport viewport.Model

	// PR mode (3 columns / 4 panels)
	centerTabIndex int // 0=Files, 1=Diff, 2=Jira Description
	rightTabIndex  int // 0=PR Comments, 1=Jira Comments, 2=Jira History
	centerViewport viewport.Model
	rightViewport  viewport.Model

	width  int
	height int
}

type detailLayout struct {
	leftW       int
	centerW     int
	rightW      int
	colH        int
	leftTopH    int
	leftBottomH int
}

type Rect struct {
	X int
	Y int
	W int
	H int
}

type DetailRects struct {
	Issue  Rect
	PR     Rect
	Desc   Rect
	Center Rect
	Right  Rect
	Help   Rect
}

func NewDetailModel(issue *api.Issue, w, h int, hasPR bool) DetailModel {
	m := DetailModel{
		issue:  issue,
		hasPR:  hasPR,
		width:  w,
		height: h,
	}

	if hasPR {
		m.focusedPanel = panelCenter
		m.centerTabIndex = 0
		m.rightTabIndex = 0
		m.prModel = NewPRDetailModel(w, h)
	} else {
		m.focusedPanel = panelDescNoPR
	}

	m = m.setSize(w, h)
	return m
}

func (m DetailModel) numPanels() int {
	if m.hasPR {
		return 4
	}
	return 2
}

func noPRColumnWidths(totalW int) (leftW, rightW int) {
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

// Legacy layout helpers kept for PRDetailModel sizing compatibility.
func columnWidths(totalW int) (leftW, rightW int) {
	return noPRColumnWidths(totalW)
}

func panelHeights(totalH int, hasPR bool) (leftTopH, rightTopH, bottomH int) {
	usable := totalH - 1
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

func computeDetailRects(w, h int, hasPR bool) DetailRects {
	helpH := 1
	contentH := maxInt(1, h-helpH)

	if !hasPR {
		leftW, rightW := noPRColumnWidths(w)
		return DetailRects{
			Issue: Rect{X: 0, Y: 0, W: leftW, H: contentH},
			Desc:  Rect{X: leftW + 1, Y: 0, W: rightW, H: contentH},
			Help:  Rect{X: 0, Y: contentH, W: w, H: helpH},
		}
	}

	l := layoutForPR(w, h)
	centerX := l.leftW + 1
	rightX := centerX + l.centerW + 1

	return DetailRects{
		Issue:  Rect{X: 0, Y: 0, W: l.leftW, H: l.leftTopH},
		PR:     Rect{X: 0, Y: l.leftTopH, W: l.leftW, H: l.leftBottomH},
		Center: Rect{X: centerX, Y: 0, W: l.centerW, H: l.colH},
		Right:  Rect{X: rightX, Y: 0, W: l.rightW, H: l.colH},
		Help:   Rect{X: 0, Y: l.colH, W: w, H: helpH},
	}
}

func layoutForPR(totalW, totalH int) detailLayout {
	usableW := totalW - 2 // two inter-column spaces
	if usableW < 3 {
		usableW = 3
	}

	leftMin, centerMin, rightMin := 24, 32, 24
	leftW := usableW * 24 / 100
	centerW := usableW * 46 / 100
	rightW := usableW - leftW - centerW

	if usableW >= leftMin+centerMin+rightMin {
		if leftW < leftMin {
			leftW = leftMin
		}
		if centerW < centerMin {
			centerW = centerMin
		}
		rightW = usableW - leftW - centerW
		if rightW < rightMin {
			deficit := rightMin - rightW
			cut := deficit
			if centerW-cut < centerMin {
				cut = centerW - centerMin
			}
			centerW -= cut
			deficit -= cut
			if deficit > 0 {
				cut = deficit
				if leftW-cut < leftMin {
					cut = leftW - leftMin
				}
				leftW -= cut
			}
			rightW = usableW - leftW - centerW
		}
	} else {
		leftW = maxInt(1, usableW*28/100)
		centerW = maxInt(1, usableW*44/100)
		rightW = usableW - leftW - centerW
		if rightW < 1 {
			rightW = 1
			if centerW > 1 {
				centerW--
			} else if leftW > 1 {
				leftW--
			}
		}
	}

	usableH := totalH - 1 // help bar
	if usableH < 4 {
		usableH = 4
	}
	leftTopH := usableH * 55 / 100
	if leftTopH < 3 {
		leftTopH = 3
	}
	leftBottomH := usableH - leftTopH
	if leftBottomH < 2 {
		leftBottomH = 2
		leftTopH = usableH - leftBottomH
	}

	return detailLayout{
		leftW:       leftW,
		centerW:     centerW,
		rightW:      rightW,
		colH:        usableH,
		leftTopH:    leftTopH,
		leftBottomH: leftBottomH,
	}
}

func (m DetailModel) setSize(w, h int) DetailModel {
	m.width = w
	m.height = h
	rects := computeDetailRects(w, h, m.hasPR)

	if !m.hasPR {
		leftInnerW, leftInnerH := panelInnerSize(rects.Issue.W, rects.Issue.H)
		issueVpH := maxInt(1, leftInnerH-2)
		m.issueViewport.Width = leftInnerW
		m.issueViewport.Height = issueVpH

		rightInnerW, rightInnerH := panelInnerSize(rects.Desc.W, rects.Desc.H)
		descVpH := maxInt(1, rightInnerH-3) // title + tabs + divider
		m.descViewport.Width = rightInnerW
		m.descViewport.Height = descVpH
		if m.issue != nil {
			m.refreshIssueViewport()
			m.refreshNoPRDescViewport()
		}
		return m
	}

	leftTopInnerW, leftTopInnerH := panelInnerSize(rects.Issue.W, rects.Issue.H)
	issueVpH := maxInt(1, leftTopInnerH-2)
	m.issueViewport.Width = leftTopInnerW
	m.issueViewport.Height = issueVpH

	leftBottomInnerW, leftBottomInnerH := panelInnerSize(rects.PR.W, rects.PR.H)
	prInfoVpH := maxInt(1, leftBottomInnerH-2)
	m.prInfoViewport.Width = leftBottomInnerW
	m.prInfoViewport.Height = prInfoVpH

	centerInnerW, centerInnerH := panelInnerSize(rects.Center.W, rects.Center.H)
	centerVpH := maxInt(1, centerInnerH-3)
	m.centerViewport.Width = centerInnerW
	m.centerViewport.Height = centerVpH

	rightInnerW, rightInnerH := panelInnerSize(rects.Right.W, rects.Right.H)
	rightVpH := maxInt(1, rightInnerH-3)
	m.rightViewport.Width = rightInnerW
	m.rightViewport.Height = rightVpH

	if m.issue != nil {
		m.refreshIssueViewport()
		m.refreshPRInfoViewport()
		m.refreshCenterViewport()
		m.refreshRightViewport()
	}
	if m.hasPR {
		m.prModel = m.prModel.setSize(w, h, rects.PR.H)
	}
	return m
}

func (m DetailModel) update(msg tea.Msg) (DetailModel, tea.Cmd) {
	var cmd tea.Cmd
	if !m.hasPR {
		if m.focusedPanel == panelIssueInfo {
			m.issueViewport, cmd = m.issueViewport.Update(msg)
			return m, cmd
		}
		if m.focusedPanel == panelDescNoPR {
			m.descViewport, cmd = m.descViewport.Update(msg)
		}
		return m, cmd
	}

	switch m.focusedPanel {
	case panelIssueInfo:
		m.issueViewport, cmd = m.issueViewport.Update(msg)
	case panelPRInfo:
		m.prInfoViewport, cmd = m.prInfoViewport.Update(msg)
	case panelCenter:
		m.centerViewport, cmd = m.centerViewport.Update(msg)
	case panelRight:
		m.rightViewport, cmd = m.rightViewport.Update(msg)
	}
	return m, cmd
}

func (m DetailModel) view() string {
	if m.issue == nil {
		return "Loading..."
	}

	helpBar := helpStyle.Width(m.width).Render(
		"  " + keyStyle.Render("Tab/S-Tab") + " panel  " +
			keyStyle.Render("1-"+fmt.Sprintf("%d", m.numPanels())) + " jump  " +
			keyStyle.Render("[/]") + " tab  " +
			keyStyle.Render("↑/↓") + " scroll  " +
			keyStyle.Render("t") + " transition  " +
			keyStyle.Render("c") + " comment  " +
			keyStyle.Render("a") + " assign  " +
			keyStyle.Render("e") + " edit  " +
			keyStyle.Render("o") + " browser  " +
			keyStyle.Render("q") + " back",
	)
	rects := computeDetailRects(m.width, m.height, m.hasPR)

	if !m.hasPR {
		issuePanel := m.renderIssueInfoPanel(rects.Issue.W, rects.Issue.H, m.focusedPanel == panelIssueInfo)
		descPanel := m.renderNoPRDescriptionPanel(rects.Desc.W, rects.Desc.H, m.focusedPanel == panelDescNoPR)

		leftLines := normalizeBlock(issuePanel, rects.Issue.W, rects.Issue.H)
		rightLines := normalizeBlock(descPanel, rects.Desc.W, rects.Desc.H)
		rowLines := make([]string, rects.Issue.H)
		for i := 0; i < rects.Issue.H; i++ {
			rowLines[i] = leftLines[i] + " " + rightLines[i]
		}
		row := strings.Join(rowLines, "\n")
		return lipgloss.JoinVertical(lipgloss.Left, row, helpBar)
	}

	issuePanel := m.renderIssueInfoPanel(rects.Issue.W, rects.Issue.H, m.focusedPanel == panelIssueInfo)
	prPanel := m.renderPRInfoPanel(rects.PR.W, rects.PR.H, m.focusedPanel == panelPRInfo)
	leftCol := lipgloss.JoinVertical(lipgloss.Left, issuePanel, prPanel)

	centerPanel := m.renderCenterPanel(rects.Center.W, rects.Center.H, m.focusedPanel == panelCenter)
	rightPanel := m.renderRightPanel(rects.Right.W, rects.Right.H, m.focusedPanel == panelRight)

	leftColLines := normalizeBlock(leftCol, rects.Issue.W, rects.Center.H)
	centerLines := normalizeBlock(centerPanel, rects.Center.W, rects.Center.H)
	rightLines := normalizeBlock(rightPanel, rects.Right.W, rects.Right.H)
	rowLines := make([]string, rects.Center.H)
	for i := 0; i < rects.Center.H; i++ {
		rowLines[i] = leftColLines[i] + " " + centerLines[i] + " " + rightLines[i]
	}
	row := strings.Join(rowLines, "\n")
	return lipgloss.JoinVertical(lipgloss.Left, row, helpBar)
}

func (m DetailModel) renderIssueInfoPanel(outerW, outerH int, active bool) string {
	innerW, innerH := panelInnerSize(outerW, outerH)
	content := renderPanelScaffold(1, "Issue Info", active, nil, 0, innerW, m.issueViewport.View())

	style := panelInactiveStyle
	if active {
		style = panelActiveStyle
	}
	return style.Width(innerW).Height(innerH).Render(content)
}

func renderIssueInfoContent(issue *api.Issue, innerW int) string {
	var sb strings.Builder

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
	return sb.String()
}

func (m DetailModel) renderPRInfoPanel(outerW, outerH int, active bool) string {
	innerW, innerH := panelInnerSize(outerW, outerH)
	content := renderPanelScaffold(2, "Pull Request", active, nil, 0, innerW, m.prInfoViewport.View())

	style := panelInactiveStyle
	if active {
		style = panelActiveStyle
	}
	return style.Width(innerW).Height(innerH).Render(content)
}

func (m DetailModel) renderNoPRDescriptionPanel(outerW, outerH int, active bool) string {
	innerW, innerH := panelInnerSize(outerW, outerH)

	tabs := []string{"Description", "Comments"}
	content := renderPanelScaffold(panelDescNoPR+1, "Description", active, tabs, m.descTabIndex, innerW, m.descViewport.View())

	style := panelInactiveStyle
	if active {
		style = panelActiveStyle
	}
	return style.Width(innerW).Height(innerH).Render(content)
}

func (m DetailModel) renderCenterPanel(outerW, outerH int, active bool) string {
	innerW, innerH := panelInnerSize(outerW, outerH)

	tabs := []string{"Files", "Diff", "Jira Description"}
	content := renderPanelScaffold(3, "Changes", active, tabs, m.centerTabIndex, innerW, m.centerViewport.View())

	style := panelInactiveStyle
	if active {
		style = panelActiveStyle
	}
	return style.Width(innerW).Height(innerH).Render(content)
}

func (m DetailModel) renderRightPanel(outerW, outerH int, active bool) string {
	innerW, innerH := panelInnerSize(outerW, outerH)

	tabs := []string{"PR Comments", "Jira Comments", "Jira History"}
	content := renderPanelScaffold(4, "Discussion", active, tabs, m.rightTabIndex, innerW, m.rightViewport.View())

	style := panelInactiveStyle
	if active {
		style = panelActiveStyle
	}
	return style.Width(innerW).Height(innerH).Render(content)
}

func renderDescContent(issue *api.Issue, width int) string {
	var sb strings.Builder

	if issue.Fields.Description != nil {
		desc := strings.TrimSpace(adf.Render(issue.Fields.Description))
		if desc != "" {
			sb.WriteString(indentWrappedText(desc, maxInt(1, width-2), 2))
		} else {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render("  (no description)"))
		}
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render("  (no description)"))
	}
	return sb.String()
}

func renderCommentsContent(issue *api.Issue, width int) string {
	if issue.Fields.Comment == nil || len(issue.Fields.Comment.Comments) == 0 {
		return lipgloss.NewStyle().Foreground(colorMuted).Render("  (no comments)")
	}

	var sb strings.Builder
	metaW := maxInt(1, width-2)
	for _, comment := range issue.Fields.Comment.Comments {
		author := "Unknown"
		if comment.Author != nil {
			author = comment.Author.DisplayName
		}
		meta := fmt.Sprintf("%s  ·  %s", author, formatTime(comment.Created.Time))
		for _, line := range wrapLine(meta, metaW) {
			sb.WriteString("  " + commentMetaStyle.Render(line) + "\n")
		}

		body := strings.TrimSpace(adf.Render(comment.Body))
		if body != "" {
			sb.WriteString(indentWrappedText(body, maxInt(1, width-2), 2))
		}
		sb.WriteString("\n")
		sb.WriteString(strings.Repeat("─", maxInt(1, width-4)) + "\n")
	}
	return sb.String()
}

func renderJiraHistoryContent(issue *api.Issue, width int) string {
	if issue == nil || issue.Changelog == nil || len(issue.Changelog.Histories) == 0 {
		return lipgloss.NewStyle().Foreground(colorMuted).Render("  (no issue history)")
	}

	histories := make([]api.IssueHistory, len(issue.Changelog.Histories))
	copy(histories, issue.Changelog.Histories)
	sort.Slice(histories, func(i, j int) bool {
		return histories[i].Created.Time.After(histories[j].Created.Time)
	})

	var sb strings.Builder
	for _, h := range histories {
		author := "Unknown"
		if h.Author != nil && h.Author.DisplayName != "" {
			author = h.Author.DisplayName
		}
		timeLabel := formatTime(h.Created.Time)

		prioritized, extra := compactHistoryItems(h.Items)
		if len(prioritized) == 0 {
			continue
		}

		meta := fmt.Sprintf("%s  ·  %s", author, timeLabel)
		for _, line := range wrapLine(meta, maxInt(1, width-2)) {
			sb.WriteString("  " + commentMetaStyle.Render(line) + "\n")
		}
		for _, item := range prioritized {
			fromVal := normalizeHistoryValue(item.FromString)
			toVal := normalizeHistoryValue(item.ToString)
			line := fmt.Sprintf("%s: %s -> %s", item.Field, fromVal, toVal)
			sb.WriteString(indentWrappedText(line, maxInt(1, width-4), 4))
		}
		if extra > 0 {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(fmt.Sprintf("    (+%d more changes)", extra)) + "\n")
		}
		sb.WriteString("\n")
	}

	if strings.TrimSpace(sb.String()) == "" {
		return lipgloss.NewStyle().Foreground(colorMuted).Render("  (no issue history)")
	}
	return sb.String()
}

func compactHistoryItems(items []api.IssueHistoryItem) ([]api.IssueHistoryItem, int) {
	if len(items) == 0 {
		return nil, 0
	}

	priority := map[string]int{
		"status":   0,
		"assignee": 1,
		"priority": 2,
		"summary":  3,
	}

	sorted := make([]api.IssueHistoryItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		fi := strings.ToLower(strings.TrimSpace(sorted[i].Field))
		fj := strings.ToLower(strings.TrimSpace(sorted[j].Field))
		pi, iok := priority[fi]
		pj, jok := priority[fj]
		switch {
		case iok && jok:
			return pi < pj
		case iok:
			return true
		case jok:
			return false
		default:
			return fi < fj
		}
	})

	limit := 4
	if len(sorted) < limit {
		limit = len(sorted)
	}
	return sorted[:limit], len(sorted) - limit
}

func normalizeHistoryValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "—"
	}
	return v
}

func (m *DetailModel) refreshNoPRDescViewport() {
	rects := computeDetailRects(m.width, m.height, false)
	innerW, _ := panelInnerSize(rects.Desc.W, rects.Desc.H)
	if m.descTabIndex == 0 {
		m.descViewport.SetContent(renderDescContent(m.issue, innerW))
	} else {
		m.descViewport.SetContent(renderCommentsContent(m.issue, innerW))
	}
	m.descViewport.GotoTop()
}

func (m *DetailModel) refreshIssueViewport() {
	innerW := maxInt(1, m.issueViewport.Width)
	m.issueViewport.SetContent(renderIssueInfoContent(m.issue, innerW))
	m.issueViewport.GotoTop()
}

func (m *DetailModel) refreshPRInfoViewport() {
	innerW := maxInt(1, m.prInfoViewport.Width)
	m.prInfoViewport.SetContent(m.prModel.renderOverviewContent(innerW))
	m.prInfoViewport.GotoTop()
}

func (m *DetailModel) refreshCenterViewport() {
	rects := computeDetailRects(m.width, m.height, true)
	innerW, _ := panelInnerSize(rects.Center.W, rects.Center.H)
	if m.prModel.loading && m.centerTabIndex != 2 {
		m.centerViewport.SetContent(lipgloss.NewStyle().Foreground(colorMuted).Render("Loading pull request data..."))
		m.centerViewport.GotoTop()
		return
	}
	if m.prModel.err != nil && m.centerTabIndex != 2 {
		m.centerViewport.SetContent(lipgloss.NewStyle().Foreground(colorRed).Render("Error: " + m.prModel.err.Error()))
		m.centerViewport.GotoTop()
		return
	}
	switch m.centerTabIndex {
	case 0:
		m.centerViewport.SetContent(renderFilesTab(m.prModel.fileDiffs))
	case 1:
		m.centerViewport.SetContent(renderDiffTab(m.prModel.fileDiffs))
	default:
		m.centerViewport.SetContent(renderDescContent(m.issue, innerW))
	}
	m.centerViewport.GotoTop()
}

func (m *DetailModel) refreshRightViewport() {
	rects := computeDetailRects(m.width, m.height, true)
	innerW, _ := panelInnerSize(rects.Right.W, rects.Right.H)
	if m.prModel.loading && m.rightTabIndex == 0 {
		m.rightViewport.SetContent(lipgloss.NewStyle().Foreground(colorMuted).Render("Loading pull request data..."))
		m.rightViewport.GotoTop()
		return
	}
	if m.prModel.err != nil && m.rightTabIndex == 0 {
		m.rightViewport.SetContent(lipgloss.NewStyle().Foreground(colorRed).Render("Error: " + m.prModel.err.Error()))
		m.rightViewport.GotoTop()
		return
	}
	switch m.rightTabIndex {
	case 0:
		m.rightViewport.SetContent(renderPRCommentsTab(m.prModel.threads, innerW))
	case 1:
		m.rightViewport.SetContent(renderCommentsContent(m.issue, innerW))
	default:
		m.rightViewport.SetContent(renderJiraHistoryContent(m.issue, innerW))
	}
	m.rightViewport.GotoTop()
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

func indentWrappedText(text string, width int, spaces int) string {
	if width < 1 {
		width = 1
	}
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(text, "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			out = append(out, indent)
			continue
		}
		wrapped := wrapLine(line, width)
		for _, w := range wrapped {
			out = append(out, indent+w)
		}
	}
	return strings.Join(out, "\n") + "\n"
}

func wrapLine(line string, width int) []string {
	if width < 1 {
		return []string{line}
	}
	if lipgloss.Width(line) <= width {
		return []string{line}
	}

	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		candidate := cur + " " + w
		if lipgloss.Width(candidate) <= width {
			cur = candidate
			continue
		}
		lines = append(lines, cur)
		if lipgloss.Width(w) <= width {
			cur = w
			continue
		}
		for lipgloss.Width(w) > width {
			cut := cutToWidth(w, width)
			lines = append(lines, cut)
			w = strings.TrimPrefix(w, cut)
		}
		cur = w
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func cutToWidth(s string, width int) string {
	if width < 1 {
		return s
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	for i := range runes {
		part := string(runes[:i+1])
		if lipgloss.Width(part) > width {
			if i == 0 {
				return string(runes[:1])
			}
			return string(runes[:i])
		}
	}
	return s
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func normalizeBlock(block string, width, height int) []string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	raw := strings.Split(block, "\n")
	for len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}

	lines := make([]string, height)
	for i := 0; i < height; i++ {
		if i < len(raw) {
			line := raw[i]
			if !strings.Contains(line, "\x1b[") && lipgloss.Width(line) > width {
				line = cutToWidth(line, width)
			}
			lines[i] = padToWidth(line, width)
		} else {
			lines[i] = strings.Repeat(" ", width)
		}
	}
	return lines
}

func padToWidth(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
