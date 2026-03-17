package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/api"
)

// PRDetailModel renders PR data across two panels: overview (left) and files/diff (right).
type PRDetailModel struct {
	pr        *api.PullRequest
	build     *api.Build
	fileDiffs []api.FileDiff
	reviewers []api.Reviewer
	threads   []api.PRCommentThread

	// Files/Diff/Comments panel
	filesTabIndex int // 0=Files, 1=Diff, 2=Comments
	filesViewport viewport.Model

	loading  bool
	notFound bool
	err      error
	width    int
	height   int
}

// NewPRDetailModel creates a loading PR detail model.
func NewPRDetailModel(w, h int) PRDetailModel {
	_, _, botH := panelHeights(h, true)
	vpH := botH - 4
	if vpH < 1 {
		vpH = 1
	}
	_, rightW := columnWidths(w)
	vp := viewport.New(rightW-4, vpH)
	vp.SetContent(lipgloss.NewStyle().Foreground(colorMuted).Padding(1, 2).Render("Fetching pull request data..."))
	return PRDetailModel{
		loading:       true,
		filesViewport: vp,
		width:         w,
		height:        h,
	}
}

func (m PRDetailModel) setData(pr *api.PullRequest, build *api.Build, fileDiffs []api.FileDiff, reviewers []api.Reviewer, threads []api.PRCommentThread) PRDetailModel {
	m.loading = false
	m.pr = pr
	m.build = build
	m.fileDiffs = fileDiffs
	m.reviewers = reviewers
	m.threads = threads
	if pr == nil {
		m.notFound = true
	}
	_, rightW := columnWidths(m.width)
	m.filesViewport.SetContent(renderFilesContent(m.filesTabIndex, fileDiffs, threads, rightW-4))
	return m
}

func (m PRDetailModel) setReviewers(reviewers []api.Reviewer) PRDetailModel {
	m.reviewers = reviewers
	_, rightW := columnWidths(m.width)
	m.filesViewport.SetContent(renderFilesContent(m.filesTabIndex, m.fileDiffs, m.threads, rightW-4))
	return m
}

func (m PRDetailModel) setThreads(threads []api.PRCommentThread) PRDetailModel {
	m.threads = threads
	_, rightW := columnWidths(m.width)
	m.filesViewport.SetContent(renderFilesContent(m.filesTabIndex, m.fileDiffs, threads, rightW-4))
	return m
}

func (m PRDetailModel) setError(err error) PRDetailModel {
	m.loading = false
	m.err = err
	m.filesViewport.SetContent(lipgloss.NewStyle().Foreground(colorRed).Padding(1, 2).Render("Error: " + err.Error()))
	return m
}

// setSize is called when the terminal is resized.
// botPanelH is the outer height for the bottom row panels.
func (m PRDetailModel) setSize(w, h, botPanelH int) PRDetailModel {
	m.width = w
	m.height = h

	_, rightW := columnWidths(w)
	vpH := botPanelH - 4
	if vpH < 1 {
		vpH = 1
	}
	m.filesViewport.Width = rightW - 4
	m.filesViewport.Height = vpH
	if !m.loading && m.err == nil {
		m.filesViewport.SetContent(renderFilesContent(m.filesTabIndex, m.fileDiffs, m.threads, rightW-4))
	}
	return m
}

// updateFilesViewport passes scroll messages to the files viewport.
func (m PRDetailModel) updateFilesViewport(msg tea.Msg) (PRDetailModel, tea.Cmd) {
	var cmd tea.Cmd
	m.filesViewport, cmd = m.filesViewport.Update(msg)
	return m, cmd
}

// renderOverviewPanel renders the compact PR overview for the left-bottom panel.
func (m PRDetailModel) renderOverviewPanel(outerW, outerH int, active bool) string {
	innerW, innerH := panelInnerSize(outerW, outerH)
	content := renderPanelScaffold(2, "Pull Request", active, nil, 0, innerW, innerH, m.renderOverviewContent(innerW))

	style := panelInactiveStyle
	if active {
		style = panelActiveStyle
	}
	return style.Width(innerW).Height(innerH).Render(content)
}

func (m PRDetailModel) renderOverviewContent(innerW int) string {
	if m.loading {
		return lipgloss.NewStyle().Foreground(colorMuted).Render("Loading...")
	}
	if m.err != nil {
		return lipgloss.NewStyle().Foreground(colorRed).Render("Error: " + TruncateString(m.err.Error(), innerW-8))
	}
	if m.notFound || m.pr == nil {
		return lipgloss.NewStyle().Foreground(colorMuted).
			Render("No linked PR found.\n\nBranch must contain\nthe Jira issue key.")
	}

	pr := m.pr
	sourceBranch := strings.TrimPrefix(pr.SourceRefName, "refs/heads/")
	targetBranch := strings.TrimPrefix(pr.TargetRefName, "refs/heads/")

	var sb strings.Builder
	sb.WriteString(fieldLabelStyle.Render("Status:") + " " + colorPRStatus(pr.Status) + "\n")
	sb.WriteString(fieldLabelStyle.Render("Author:") + " " +
		fieldValueStyle.Render(TruncateString(pr.CreatedBy.DisplayName, innerW-16)) + "\n")
	sb.WriteString(fieldLabelStyle.Render("Branch:") + " " +
		fieldValueStyle.Render(TruncateString(sourceBranch+" → "+targetBranch, innerW-16)) + "\n")

	// Pipeline
	sb.WriteString("\n")
	if m.build == nil {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render("Pipeline: none") + "\n")
	} else {
		sb.WriteString(fieldLabelStyle.Render("Pipeline:") + " " + colorBuildStatus(m.build.Status, m.build.Result) + "\n")
	}

	// Reviewers
	sb.WriteString("\n")
	sb.WriteString(sectionStyle.Render("Reviewers") + "\n")
	if len(m.reviewers) == 0 {
		sb.WriteString("  " + lipgloss.NewStyle().Foreground(colorMuted).Render("None assigned") + "\n")
	} else {
		for _, r := range m.reviewers {
			line := fmt.Sprintf("  %-20s %s",
				TruncateString(r.DisplayName, 20),
				colorVoteLabel(r.Vote),
			)
			sb.WriteString(TruncateString(line, innerW) + "\n")
		}
	}

	return sb.String()
}

// renderFilesPanel renders the PR files/diff/comments panel (right-bottom).
func (m PRDetailModel) renderFilesPanel(outerW, outerH int, active bool) string {
	innerW, innerH := panelInnerSize(outerW, outerH)

	tabs := []string{"Files", "Diff", "Comments"}

	var vpContent string
	if m.loading {
		vpContent = lipgloss.NewStyle().Foreground(colorMuted).Render("Loading...")
	} else {
		vpContent = m.filesViewport.View()
	}
	content := renderPanelScaffold(4, "PR Files", active, tabs, m.filesTabIndex, innerW, innerH, vpContent)

	style := panelInactiveStyle
	if active {
		style = panelActiveStyle
	}
	return style.Width(innerW).Height(innerH).Render(content)
}

// refreshFilesViewport rebuilds the files viewport content after a tab switch.
func (m *PRDetailModel) refreshFilesViewport() {
	_, rightW := columnWidths(m.width)
	innerW := rightW - 4
	if innerW < 1 {
		innerW = 1
	}
	m.filesViewport.SetContent(renderFilesContent(m.filesTabIndex, m.fileDiffs, m.threads, innerW))
	m.filesViewport.GotoTop()
}

// renderFilesContent returns viewport content for the given tab index.
func renderFilesContent(tabIndex int, fileDiffs []api.FileDiff, threads []api.PRCommentThread, width int) string {
	switch tabIndex {
	case 0:
		return renderFilesTab(fileDiffs, width)
	case 1:
		return renderDiffTab(fileDiffs, width)
	case 2:
		return renderPRCommentsTab(threads, width)
	}
	return ""
}

func renderFilesTab(fileDiffs []api.FileDiff, width int) string {
	if len(fileDiffs) == 0 {
		return lipgloss.NewStyle().Foreground(colorMuted).Render("  (no changed files)")
	}
	pathW := maxInt(1, width-6)
	var sb strings.Builder
	for _, fd := range fileDiffs {
		icon := changeTypeIcon(fd.ChangeType)
		path := lipgloss.NewStyle().Foreground(colorFg).Render(TruncateString(fd.Path, pathW))
		sb.WriteString(fmt.Sprintf("  %s  %s\n", icon, path))
	}
	return sb.String()
}

func renderDiffTab(fileDiffs []api.FileDiff, width int) string {
	hasDiffs := false
	for _, fd := range fileDiffs {
		if len(fd.Hunks) > 0 || fd.Binary {
			hasDiffs = true
			break
		}
	}
	if !hasDiffs {
		return lipgloss.NewStyle().Foreground(colorMuted).Render("  (no diff available)")
	}

	var sb strings.Builder
	for _, fd := range fileDiffs {
		if len(fd.Hunks) == 0 && !fd.Binary {
			continue
		}
		headerText := "┌ " + changeTypeLabel(fd.ChangeType) + ": " + fd.Path
		headerW := maxInt(1, width-1)
		for _, headerLine := range wrapLinePreserveWhitespace(headerText, headerW) {
			fileHeader := lipgloss.NewStyle().
				Foreground(colorSecond).
				Bold(true).
				Render(headerLine)
			sb.WriteString(fileHeader + "\n")
		}

		if fd.Binary {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render("  [binary file]") + "\n\n")
			continue
		}
		for _, hunk := range fd.Hunks {
			hunkHeaderW := maxInt(1, width)
			for _, hunkLine := range wrapLinePreserveWhitespace(hunk.Header, hunkHeaderW) {
				sb.WriteString(lipgloss.NewStyle().Foreground(colorSecond).Render(hunkLine) + "\n")
			}
			lineW := maxInt(1, width-1)
			for _, line := range hunk.Lines {
				content := strings.ReplaceAll(line.Content, "\t", "    ")
				wrapped := wrapLinePreserveWhitespace(content, lineW)
				if len(wrapped) == 0 {
					wrapped = []string{""}
				}

				for i, segment := range wrapped {
					prefix := " "
					if i == 0 {
						switch line.Type {
						case "added":
							prefix = "+"
						case "deleted":
							prefix = "-"
						}
					} else {
						segment = strings.TrimLeft(segment, " ")
					}

					rendered := prefix + segment
					switch line.Type {
					case "added":
						sb.WriteString(lipgloss.NewStyle().Foreground(colorGreen).Render(rendered) + "\n")
					case "deleted":
						sb.WriteString(lipgloss.NewStyle().Foreground(colorRed).Render(rendered) + "\n")
					default:
						sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(rendered) + "\n")
					}
				}
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderPRCommentsTab(threads []api.PRCommentThread, width int) string {
	if len(threads) == 0 {
		return lipgloss.NewStyle().Foreground(colorMuted).Render("  (no comments yet  •  press c to add one)")
	}
	var sb strings.Builder
	bodyW := maxInt(1, width-4)
	for i, t := range threads {
		if len(t.Comments) == 0 {
			continue
		}
		root := t.Comments[0]
		typeLabel := threadTypeLabel(t)
		header := fmt.Sprintf("[%d] %s  •  %s  •  %s",
			i+1,
			lipgloss.NewStyle().Foreground(colorSecond).Bold(true).Render(typeLabel),
			lipgloss.NewStyle().Foreground(colorFg).Render(root.Author.DisplayName),
			lipgloss.NewStyle().Foreground(colorMuted).Render(formatTime(root.PublishedDate)),
		)
		sb.WriteString("  " + TruncateString(header, maxInt(1, width-2)) + "\n")

		body := strings.ReplaceAll(root.Content, "\n", " ")
		body = strings.TrimSpace(body)
		if body != "" {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorFg).Render(indentWrappedText(body, bodyW, 4)))
		}

		for _, reply := range t.Comments[1:] {
			if reply.IsDeleted || reply.CommentType == "system" {
				continue
			}
			replyBody := strings.ReplaceAll(reply.Content, "\n", " ")
			replyLine := fmt.Sprintf("↳ %s  •  %s",
				lipgloss.NewStyle().Foreground(colorFg).Render(reply.Author.DisplayName),
				lipgloss.NewStyle().Foreground(colorMuted).Render(formatTime(reply.PublishedDate)),
			)
			sb.WriteString("  " + TruncateString(replyLine, maxInt(1, width-2)) + "\n")
			replyBody = strings.TrimSpace(replyBody)
			if replyBody != "" {
				sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(indentWrappedText(replyBody, bodyW, 6)))
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// threadTypeLabel returns a short human-readable label for a PR comment thread.
func threadTypeLabel(t api.PRCommentThread) string {
	if t.ThreadContext == nil || t.ThreadContext.FilePath == "" {
		return "General"
	}
	if t.ThreadContext.RightFileStart != nil {
		return fmt.Sprintf("Code: %s:%d", t.ThreadContext.FilePath, t.ThreadContext.RightFileStart.Line)
	}
	return "File: " + t.ThreadContext.FilePath
}

func colorPRStatus(status string) string {
	switch strings.ToLower(status) {
	case "active":
		return lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render("● Active")
	case "completed":
		return lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("✓ Completed")
	case "abandoned":
		return lipgloss.NewStyle().Foreground(colorMuted).Render("○ Abandoned")
	default:
		return status
	}
}

func colorBuildStatus(status, result string) string {
	switch strings.ToLower(status) {
	case "inprogress":
		return lipgloss.NewStyle().Foreground(colorPrimary).Render("● In Progress")
	case "notstarted":
		return lipgloss.NewStyle().Foreground(colorMuted).Render("○ Not Started")
	case "completed":
		switch strings.ToLower(result) {
		case "succeeded":
			return lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("✓ Succeeded")
		case "failed":
			return lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render("✗ Failed")
		case "canceled":
			return lipgloss.NewStyle().Foreground(colorMuted).Render("○ Cancelled")
		case "partiallysucceeded":
			return lipgloss.NewStyle().Foreground(colorYellow).Render("⚠ Partially Succeeded")
		}
	}
	return status
}

func changeTypeIcon(ct string) string {
	switch strings.ToLower(ct) {
	case "add":
		return lipgloss.NewStyle().Foreground(colorGreen).Render("A")
	case "edit":
		return lipgloss.NewStyle().Foreground(colorPrimary).Render("M")
	case "delete":
		return lipgloss.NewStyle().Foreground(colorRed).Render("D")
	case "rename":
		return lipgloss.NewStyle().Foreground(colorYellow).Render("R")
	default:
		return lipgloss.NewStyle().Foreground(colorMuted).Render("?")
	}
}

func changeTypeLabel(ct string) string {
	switch strings.ToLower(ct) {
	case "add":
		return "added"
	case "edit":
		return "modified"
	case "delete":
		return "deleted"
	case "rename":
		return "renamed"
	default:
		return ct
	}
}

// colorVoteLabel returns a colored string for a reviewer's numeric vote.
func colorVoteLabel(vote int) string {
	switch vote {
	case 10:
		return lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("✓ Approved")
	case 5:
		return lipgloss.NewStyle().Foreground(colorGreen).Render("✓ w/ suggestions")
	case 0:
		return lipgloss.NewStyle().Foreground(colorMuted).Render("○ No vote")
	case -5:
		return lipgloss.NewStyle().Foreground(colorYellow).Render("⏳ Waiting")
	case -10:
		return lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render("✗ Rejected")
	default:
		return lipgloss.NewStyle().Foreground(colorMuted).Render(fmt.Sprintf("? (%d)", vote))
	}
}

// colorVoteOption returns a colored label for a vote option in the VoteModel.
func colorVoteOption(label string, vote int) string {
	switch vote {
	case 10:
		return lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render(label)
	case 5:
		return lipgloss.NewStyle().Foreground(colorGreen).Render(label)
	case 0:
		return lipgloss.NewStyle().Foreground(colorMuted).Render(label)
	case -5:
		return lipgloss.NewStyle().Foreground(colorYellow).Render(label)
	case -10:
		return lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render(label)
	default:
		return label
	}
}
