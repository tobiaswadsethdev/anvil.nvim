package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/api"
)

// PRDetailModel renders the Pull Request tab in the issue detail view.
type PRDetailModel struct {
	pr        *api.PullRequest
	build     *api.Build
	fileDiffs []api.FileDiff
	reviewers []api.Reviewer
	threads   []api.PRCommentThread
	viewport  viewport.Model
	loading   bool
	notFound  bool
	err       error
	width     int
	height    int
}

// NewPRDetailModel creates a loading PR detail model.
func NewPRDetailModel(w, h int) PRDetailModel {
	vp := viewport.New(w, prViewportHeight(h))
	vp.SetContent(lipgloss.NewStyle().Foreground(colorMuted).Padding(1, 2).Render("Fetching pull request data..."))
	return PRDetailModel{
		loading:  true,
		viewport: vp,
		width:    w,
		height:   h,
	}
}

func prViewportHeight(h int) int {
	// title(1) + tabBar(1) + statusBar(1) + helpBar(1) + padding(3)
	return h - 7
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
	m.viewport.SetContent(renderPRContent(pr, build, fileDiffs, reviewers, threads, m.width))
	return m
}

func (m PRDetailModel) setReviewers(reviewers []api.Reviewer) PRDetailModel {
	m.reviewers = reviewers
	m.viewport.SetContent(renderPRContent(m.pr, m.build, m.fileDiffs, reviewers, m.threads, m.width))
	return m
}

func (m PRDetailModel) setThreads(threads []api.PRCommentThread) PRDetailModel {
	m.threads = threads
	m.viewport.SetContent(renderPRContent(m.pr, m.build, m.fileDiffs, m.reviewers, threads, m.width))
	return m
}

func (m PRDetailModel) setError(err error) PRDetailModel {
	m.loading = false
	m.err = err
	m.viewport.SetContent(lipgloss.NewStyle().Foreground(colorRed).Padding(1, 2).Render("Error: " + err.Error()))
	return m
}

func (m PRDetailModel) setSize(w, h int) PRDetailModel {
	m.width = w
	m.height = h
	m.viewport.Width = w
	m.viewport.Height = prViewportHeight(h)
	if !m.loading && m.err == nil {
		m.viewport.SetContent(renderPRContent(m.pr, m.build, m.fileDiffs, m.reviewers, m.threads, w))
	}
	return m
}

func (m PRDetailModel) update(msg tea.Msg) (PRDetailModel, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m PRDetailModel) view() string {
	return m.viewport.View()
}

// renderPRContent builds the full scrollable content for the PR tab.
func renderPRContent(pr *api.PullRequest, build *api.Build, fileDiffs []api.FileDiff, reviewers []api.Reviewer, threads []api.PRCommentThread, width int) string {
	if pr == nil {
		return lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(2, 2).
			Render("No linked pull request found for this issue.\n\nA pull request must have a source branch\ncontaining the Jira issue key (e.g. feature/CODE-123).")
	}

	var sb strings.Builder

	// PR Header
	sb.WriteString(sectionStyle.Render("Pull Request") + "\n")

	sourceBranch := strings.TrimPrefix(pr.SourceRefName, "refs/heads/")
	targetBranch := strings.TrimPrefix(pr.TargetRefName, "refs/heads/")

	writeField(&sb, "Title", pr.Title)
	writeField(&sb, "Status", colorPRStatus(pr.Status))
	writeField(&sb, "Author", pr.CreatedBy.DisplayName)
	writeField(&sb, "Created", formatTime(pr.CreationDate))
	writeField(&sb, "Source", sourceBranch)
	writeField(&sb, "Target", "→ "+targetBranch)

	// Pipeline
	sb.WriteString("\n")
	sb.WriteString(sectionStyle.Render("Pipeline") + "\n")
	if build == nil {
		sb.WriteString("  " + lipgloss.NewStyle().Foreground(colorMuted).Render("No pipeline runs found") + "\n")
	} else {
		sb.WriteString("  " + colorBuildStatus(build.Status, build.Result) + "\n")
		if build.Definition.Name != "" {
			writeField(&sb, "  Pipeline", build.Definition.Name)
		}
		if build.BuildNumber != "" {
			writeField(&sb, "  Build", "#"+build.BuildNumber)
		}
		if !build.StartTime.IsZero() {
			writeField(&sb, "  Started", formatTime(build.StartTime))
		}
	}

	// Reviewers
	sb.WriteString("\n")
	sb.WriteString(sectionStyle.Render("Reviewers") + "\n")
	if len(reviewers) == 0 {
		sb.WriteString("  " + lipgloss.NewStyle().Foreground(colorMuted).Render("No reviewers assigned") + "\n")
	} else {
		for _, r := range reviewers {
			sb.WriteString(fmt.Sprintf("  %-30s %s\n",
				lipgloss.NewStyle().Foreground(colorFg).Render(r.DisplayName),
				colorVoteLabel(r.Vote),
			))
		}
	}

	// Changed files summary
	if len(fileDiffs) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render(fmt.Sprintf("Changed Files (%d)", len(fileDiffs))) + "\n\n")
		for _, fd := range fileDiffs {
			icon := changeTypeIcon(fd.ChangeType)
			path := lipgloss.NewStyle().Foreground(colorFg).Render(fd.Path)
			sb.WriteString(fmt.Sprintf("  %s  %s\n", icon, path))
		}
	}

	// Diffs
	hasDiffs := false
	for _, fd := range fileDiffs {
		if len(fd.Hunks) > 0 || fd.Binary {
			hasDiffs = true
			break
		}
	}
	if hasDiffs {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Diff") + "\n")
		for _, fd := range fileDiffs {
			if len(fd.Hunks) == 0 && !fd.Binary {
				continue
			}
			sb.WriteString("\n")
			// File header
			fileHeader := lipgloss.NewStyle().
				Foreground(colorSecond).
				Bold(true).
				Render("┌ " + changeTypeLabel(fd.ChangeType) + ": " + fd.Path)
			sb.WriteString(fileHeader + "\n")

			if fd.Binary {
				sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render("  [binary file]") + "\n")
				continue
			}

			for _, hunk := range fd.Hunks {
				// Hunk header
				sb.WriteString(lipgloss.NewStyle().Foreground(colorSecond).Render(hunk.Header) + "\n")
				for _, line := range hunk.Lines {
					switch line.Type {
					case "added":
						sb.WriteString(lipgloss.NewStyle().Foreground(colorGreen).Render("+"+line.Content) + "\n")
					case "deleted":
						sb.WriteString(lipgloss.NewStyle().Foreground(colorRed).Render("-"+line.Content) + "\n")
					default:
						sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(" "+line.Content) + "\n")
					}
				}
			}
		}
	}

	// Comments
	sb.WriteString("\n")
	sb.WriteString(sectionStyle.Render(fmt.Sprintf("Comments (%d)", len(threads))) + "\n")
	if len(threads) == 0 {
		sb.WriteString("  " + lipgloss.NewStyle().Foreground(colorMuted).Render("No comments yet  •  press c to add one") + "\n")
	} else {
		for i, t := range threads {
			if len(t.Comments) == 0 {
				continue
			}
			root := t.Comments[0]

			// Thread type label
			typeLabel := threadTypeLabel(t)
			header := fmt.Sprintf("[%d] %s  •  %s  •  %s",
				i+1,
				lipgloss.NewStyle().Foreground(colorSecond).Bold(true).Render(typeLabel),
				lipgloss.NewStyle().Foreground(colorFg).Render(root.Author.DisplayName),
				lipgloss.NewStyle().Foreground(colorMuted).Render(formatTime(root.PublishedDate)),
			)
			sb.WriteString("  " + header + "\n")

			// Root comment body (first 120 chars, single line)
			body := strings.ReplaceAll(root.Content, "\n", " ")
			if len(body) > 120 {
				body = body[:117] + "..."
			}
			sb.WriteString("  " + lipgloss.NewStyle().Foreground(colorFg).Render("    "+body) + "\n")

			// Replies
			for _, reply := range t.Comments[1:] {
				if reply.IsDeleted || reply.CommentType == "system" {
					continue
				}
				replyBody := strings.ReplaceAll(reply.Content, "\n", " ")
				if len(replyBody) > 100 {
					replyBody = replyBody[:97] + "..."
				}
				replyLine := fmt.Sprintf("    ↳ %s  •  %s  •  %s",
					lipgloss.NewStyle().Foreground(colorFg).Render(reply.Author.DisplayName),
					lipgloss.NewStyle().Foreground(colorMuted).Render(formatTime(reply.PublishedDate)),
					lipgloss.NewStyle().Foreground(colorMuted).Render(replyBody),
				)
				sb.WriteString("  " + replyLine + "\n")
			}
			sb.WriteString("\n")
		}
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
		return lipgloss.NewStyle().Foreground(colorGreen).Render("✓ Approved with suggestions")
	case 0:
		return lipgloss.NewStyle().Foreground(colorMuted).Render("○ No vote")
	case -5:
		return lipgloss.NewStyle().Foreground(colorYellow).Render("⏳ Waiting for author")
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
