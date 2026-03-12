package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/api"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/ui/adf"
)

// --- TransitionModel ---

// TransitionModel shows available workflow transitions.
type TransitionModel struct {
	transitions []api.Transition
	issueKey    string
	cursor      int
}

func NewTransitionModel(transitions []api.Transition, issueKey string) TransitionModel {
	return TransitionModel{
		transitions: transitions,
		issueKey:    issueKey,
		cursor:      0,
	}
}

// update returns the updated model, a cmd, and a done flag.
func (m TransitionModel) update(msg tea.Msg, client *api.Client) (TransitionModel, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.transitions)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if len(m.transitions) > 0 {
				t := m.transitions[m.cursor]
				return m, doTransitionCmd(client, m.issueKey, t.ID), true
			}
		default:
			// Allow number selection
			for i, t := range m.transitions {
				if msg.String() == fmt.Sprintf("%d", i+1) {
					return m, doTransitionCmd(client, m.issueKey, t.ID), true
				}
			}
		}
	}
	return m, nil, false
}

func (m TransitionModel) view() string {
	var sb strings.Builder
	sb.WriteString(modalTitleStyle.Render("Transition Issue: " + m.issueKey) + "\n\n")

	for i, t := range m.transitions {
		num := fmt.Sprintf("%d. ", i+1)
		label := fmt.Sprintf("%-3s %s → %s", num, t.Name, t.To.Name)
		if i == m.cursor {
			sb.WriteString(selectedItemStyle.Render(label) + "\n")
		} else {
			sb.WriteString(normalItemStyle.Render(label) + "\n")
		}
	}

	sb.WriteString("\n" + helpStyle.Render("j/k: navigate  Enter/number: select  Esc: cancel"))

	return modalStyle.Render(sb.String())
}

// --- CommentModel ---

// CommentModel handles adding a new comment.
type CommentModel struct {
	issueKey string
	textarea textarea.Model
}

func NewCommentModel(issueKey string) CommentModel {
	ta := textarea.New()
	ta.Placeholder = "Write your comment here...\nCtrl+S to submit, Esc to cancel"
	ta.SetWidth(60)
	ta.SetHeight(8)
	ta.Focus()
	return CommentModel{
		issueKey: issueKey,
		textarea: ta,
	}
}

func (m CommentModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m CommentModel) update(msg tea.Msg, client *api.Client) (CommentModel, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			text := m.textarea.Value()
			if strings.TrimSpace(text) != "" {
				body := adf.FromMarkdown(text)
				return m, addCommentCmd(client, m.issueKey, body), true
			}
			return m, nil, true // empty comment, just close
		case "esc":
			return m, nil, true
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd, false
}

func (m CommentModel) view() string {
	var sb strings.Builder
	sb.WriteString(modalTitleStyle.Render("Add Comment: " + m.issueKey) + "\n\n")
	sb.WriteString(m.textarea.View())
	sb.WriteString("\n\n" + helpStyle.Render(
		keyStyle.Render("Ctrl+S")+" submit  "+keyStyle.Render("Esc")+" cancel",
	))
	return modalStyle.Render(sb.String())
}

// --- AssignModel ---

// AssignModel handles assigning an issue to a user.
type AssignModel struct {
	users     []api.User
	filtered  []api.User
	issueKey  string
	cursor    int
	search    textinput.Model
}

func NewAssignModel(users []api.User, issueKey string) AssignModel {
	ti := textinput.New()
	ti.Placeholder = "Search user..."
	ti.Focus()
	ti.Width = 30

	// Prepend unassign option
	unassign := api.User{AccountID: "", DisplayName: "Unassigned"}
	all := append([]api.User{unassign}, users...)

	return AssignModel{
		users:    all,
		filtered: all,
		issueKey: issueKey,
		search:   ti,
	}
}

func (m AssignModel) update(msg tea.Msg, client *api.Client) (AssignModel, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if len(m.filtered) > 0 {
				user := m.filtered[m.cursor]
				return m, assignIssueCmd(client, m.issueKey, user.AccountID), true
			}
		case "j", "down":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "esc":
			return m, nil, true
		default:
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.filterUsers()
			m.cursor = 0
			return m, cmd, false
		}
	default:
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return m, cmd, false
	}
	return m, nil, false
}

func (m *AssignModel) filterUsers() {
	query := strings.ToLower(m.search.Value())
	if query == "" {
		m.filtered = m.users
		return
	}
	var result []api.User
	for _, u := range m.users {
		if strings.Contains(strings.ToLower(u.DisplayName), query) ||
			strings.Contains(strings.ToLower(u.EmailAddress), query) {
			result = append(result, u)
		}
	}
	m.filtered = result
}

func (m AssignModel) view() string {
	var sb strings.Builder
	sb.WriteString(modalTitleStyle.Render("Assign Issue: " + m.issueKey) + "\n\n")
	sb.WriteString(m.search.View() + "\n\n")

	maxVisible := 8
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}

	shown := m.filtered[start:]
	if len(shown) > maxVisible {
		shown = shown[:maxVisible]
	}

	for i, u := range shown {
		realIdx := start + i
		label := u.DisplayName
		if u.EmailAddress != "" {
			label += lipgloss.NewStyle().Foreground(colorMuted).Render(" <"+u.EmailAddress+">")
		}
		if realIdx == m.cursor {
			sb.WriteString(selectedItemStyle.Render("  "+label) + "\n")
		} else {
			sb.WriteString(normalItemStyle.Render("  "+label) + "\n")
		}
	}

	if len(m.filtered) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render("  No users found") + "\n")
	}

	sb.WriteString("\n" + helpStyle.Render(
		keyStyle.Render("j/k")+" navigate  "+
			keyStyle.Render("Enter")+" assign  "+
			keyStyle.Render("Esc")+" cancel",
	))
	return modalStyle.Render(sb.String())
}

// --- VoteModel ---

// voteOption pairs a human-readable label with the Azure DevOps vote integer.
type voteOption struct {
	label string
	value int
}

var voteOptions = []voteOption{
	{"Approve", 10},
	{"Approve with suggestions", 5},
	{"Reset vote", 0},
	{"Wait for author", -5},
	{"Reject", -10},
}

// VoteModel lets the user cast a vote on a pull request.
type VoteModel struct {
	pr     *api.PullRequest
	cursor int
}

func NewVoteModel(pr *api.PullRequest) VoteModel {
	return VoteModel{pr: pr, cursor: 0}
}

// update returns the updated model, the selected option (nil if cancelled), and a done flag.
func (m VoteModel) update(msg tea.Msg) (VoteModel, *voteOption, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(voteOptions)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			selected := voteOptions[m.cursor]
			return m, &selected, true
		case "esc":
			return m, nil, true
		default:
			for i := range voteOptions {
				if msg.String() == fmt.Sprintf("%d", i+1) {
					selected := voteOptions[i]
					return m, &selected, true
				}
			}
		}
	}
	return m, nil, false
}

func (m VoteModel) view() string {
	var sb strings.Builder
	sb.WriteString(modalTitleStyle.Render("Vote on Pull Request") + "\n\n")

	for i, opt := range voteOptions {
		num := fmt.Sprintf("%d. ", i+1)
		label := num + colorVoteOption(opt.label, opt.value)
		if i == m.cursor {
			sb.WriteString(selectedItemStyle.Render(label) + "\n")
		} else {
			sb.WriteString(normalItemStyle.Render(label) + "\n")
		}
	}

	sb.WriteString("\n" + helpStyle.Render("j/k: navigate  Enter/number: select  Esc: cancel"))
	return modalStyle.Render(sb.String())
}

// --- PRCommentModel ---

// prCommentStep tracks which step of the PR comment flow the user is on.
type prCommentStep int

const (
	prCommentStepType   prCommentStep = iota // choose comment type
	prCommentStepFile                        // enter file path (File/Code)
	prCommentStepLine                        // enter line number (Code only)
	prCommentStepThread                      // select thread to reply to (Reply)
	prCommentStepText                        // enter comment body
)

// prCommentType is the kind of PR comment being created.
type prCommentType int

const (
	prCommentTypeGeneral prCommentType = iota
	prCommentTypeFile
	prCommentTypeCode
	prCommentTypeReply
)

var prCommentTypeOptions = []string{
	"General comment",
	"File comment",
	"Code comment",
	"Reply to thread",
}

// PRCommentResult holds the completed form data returned when the user submits.
type PRCommentResult struct {
	Content  string
	FilePath string // empty for general
	Line     int    // 0 for general/file
	ThreadID int    // non-zero for reply
	ParentID int    // non-zero for reply (root comment ID of chosen thread)
}

// PRCommentModel is a multi-step modal for adding PR comments and replies.
type PRCommentModel struct {
	step         prCommentStep
	commentType  prCommentType
	typeCursor   int
	threads      []api.PRCommentThread
	threadCursor int
	filePaths    []string // hint: paths from PR changed files
	filePath     textinput.Model
	lineNum      textinput.Model
	text         textarea.Model
	prevStep     prCommentStep // for Esc back-navigation
}

// NewPRCommentModel creates a new PR comment modal.
// threads: existing threads (for Reply option)
// filePaths: file paths from the PR diff (shown as hints for File/Code)
func NewPRCommentModel(threads []api.PRCommentThread, filePaths []string) PRCommentModel {
	fp := textinput.New()
	fp.Placeholder = "e.g. /src/main.go"
	fp.Width = 50
	fp.Focus()

	ln := textinput.New()
	ln.Placeholder = "e.g. 42"
	ln.Width = 10

	ta := textarea.New()
	ta.Placeholder = "Write your comment here...\nCtrl+S to submit, Esc to go back"
	ta.SetWidth(60)
	ta.SetHeight(8)

	return PRCommentModel{
		step:      prCommentStepType,
		threads:   threads,
		filePaths: filePaths,
		filePath:  fp,
		lineNum:   ln,
		text:      ta,
	}
}

// update processes a key message and returns the updated model, a result (when done), and a done flag.
func (m PRCommentModel) update(msg tea.Msg) (PRCommentModel, *PRCommentResult, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case prCommentStepType:
			return m.updateTypeStep(msg)
		case prCommentStepFile:
			return m.updateFileStep(msg)
		case prCommentStepLine:
			return m.updateLineStep(msg)
		case prCommentStepThread:
			return m.updateThreadStep(msg)
		case prCommentStepText:
			return m.updateTextStep(msg)
		}
	}
	// Delegate textarea updates for non-key messages
	if m.step == prCommentStepText {
		var cmd tea.Cmd
		m.text, cmd = m.text.Update(msg)
		_ = cmd
	}
	return m, nil, false
}

func (m PRCommentModel) updateTypeStep(msg tea.KeyMsg) (PRCommentModel, *PRCommentResult, bool) {
	switch msg.String() {
	case "j", "down":
		if m.typeCursor < len(prCommentTypeOptions)-1 {
			m.typeCursor++
		}
	case "k", "up":
		if m.typeCursor > 0 {
			m.typeCursor--
		}
	case "esc":
		return m, nil, true // cancel
	case "enter":
		return m.confirmType()
	default:
		for i := range prCommentTypeOptions {
			if msg.String() == fmt.Sprintf("%d", i+1) {
				m.typeCursor = i
				return m.confirmType()
			}
		}
	}
	return m, nil, false
}

func (m PRCommentModel) confirmType() (PRCommentModel, *PRCommentResult, bool) {
	m.commentType = prCommentType(m.typeCursor)
	switch m.commentType {
	case prCommentTypeGeneral:
		m.step = prCommentStepText
		m.text.Focus()
	case prCommentTypeFile:
		m.step = prCommentStepFile
		m.filePath.Focus()
	case prCommentTypeCode:
		m.step = prCommentStepFile
		m.filePath.Focus()
	case prCommentTypeReply:
		if len(m.threads) == 0 {
			// No threads to reply to; go back
			return m, nil, false
		}
		m.step = prCommentStepThread
		m.threadCursor = 0
	}
	return m, nil, false
}

func (m PRCommentModel) updateFileStep(msg tea.KeyMsg) (PRCommentModel, *PRCommentResult, bool) {
	switch msg.String() {
	case "esc":
		m.step = prCommentStepType
		m.filePath.Blur()
		return m, nil, false
	case "enter":
		if strings.TrimSpace(m.filePath.Value()) == "" {
			return m, nil, false
		}
		if m.commentType == prCommentTypeCode {
			m.step = prCommentStepLine
			m.filePath.Blur()
			m.lineNum.Focus()
		} else {
			m.step = prCommentStepText
			m.filePath.Blur()
			m.text.Focus()
		}
		return m, nil, false
	}
	var cmd tea.Cmd
	m.filePath, cmd = m.filePath.Update(msg)
	_ = cmd
	return m, nil, false
}

func (m PRCommentModel) updateLineStep(msg tea.KeyMsg) (PRCommentModel, *PRCommentResult, bool) {
	switch msg.String() {
	case "esc":
		m.step = prCommentStepFile
		m.lineNum.Blur()
		m.filePath.Focus()
		return m, nil, false
	case "enter":
		m.step = prCommentStepText
		m.lineNum.Blur()
		m.text.Focus()
		return m, nil, false
	}
	// Only allow digits
	if len(msg.String()) == 1 && (msg.String() >= "0" && msg.String() <= "9") {
		var cmd tea.Cmd
		m.lineNum, cmd = m.lineNum.Update(msg)
		_ = cmd
	} else if msg.String() == "backspace" {
		var cmd tea.Cmd
		m.lineNum, cmd = m.lineNum.Update(msg)
		_ = cmd
	}
	return m, nil, false
}

func (m PRCommentModel) updateThreadStep(msg tea.KeyMsg) (PRCommentModel, *PRCommentResult, bool) {
	switch msg.String() {
	case "j", "down":
		if m.threadCursor < len(m.threads)-1 {
			m.threadCursor++
		}
	case "k", "up":
		if m.threadCursor > 0 {
			m.threadCursor--
		}
	case "esc":
		m.step = prCommentStepType
		return m, nil, false
	case "enter":
		m.step = prCommentStepText
		m.text.Focus()
		return m, nil, false
	default:
		for i := range m.threads {
			if msg.String() == fmt.Sprintf("%d", i+1) {
				m.threadCursor = i
				m.step = prCommentStepText
				m.text.Focus()
				return m, nil, false
			}
		}
	}
	return m, nil, false
}

func (m PRCommentModel) updateTextStep(msg tea.KeyMsg) (PRCommentModel, *PRCommentResult, bool) {
	switch msg.String() {
	case "ctrl+s":
		content := strings.TrimSpace(m.text.Value())
		if content == "" {
			return m, nil, true // empty — close without submitting
		}
		result := m.buildResult(content)
		return m, result, true
	case "esc":
		// Go back to previous step
		m.text.Blur()
		switch m.commentType {
		case prCommentTypeGeneral:
			m.step = prCommentStepType
		case prCommentTypeFile:
			m.step = prCommentStepFile
			m.filePath.Focus()
		case prCommentTypeCode:
			m.step = prCommentStepLine
			m.lineNum.Focus()
		case prCommentTypeReply:
			m.step = prCommentStepThread
		}
		return m, nil, false
	}
	var cmd tea.Cmd
	m.text, cmd = m.text.Update(msg)
	_ = cmd
	return m, nil, false
}

func (m PRCommentModel) buildResult(content string) *PRCommentResult {
	r := &PRCommentResult{Content: content}
	switch m.commentType {
	case prCommentTypeFile:
		r.FilePath = strings.TrimSpace(m.filePath.Value())
	case prCommentTypeCode:
		r.FilePath = strings.TrimSpace(m.filePath.Value())
		r.Line, _ = strconv.Atoi(strings.TrimSpace(m.lineNum.Value()))
	case prCommentTypeReply:
		if m.threadCursor < len(m.threads) {
			t := m.threads[m.threadCursor]
			r.ThreadID = t.ID
			if len(t.Comments) > 0 {
				r.ParentID = t.Comments[0].ID
			}
		}
	}
	return r
}

func (m PRCommentModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m PRCommentModel) view() string {
	var sb strings.Builder

	switch m.step {
	case prCommentStepType:
		sb.WriteString(modalTitleStyle.Render("Add PR Comment") + "\n\n")
		for i, opt := range prCommentTypeOptions {
			label := fmt.Sprintf("%d. %s", i+1, opt)
			if i == m.typeCursor {
				sb.WriteString(selectedItemStyle.Render(label) + "\n")
			} else {
				sb.WriteString(normalItemStyle.Render(label) + "\n")
			}
		}
		if m.commentType == prCommentTypeReply && len(m.threads) == 0 {
			sb.WriteString("\n" + lipgloss.NewStyle().Foreground(colorMuted).Render("  (no threads to reply to)") + "\n")
		}
		sb.WriteString("\n" + helpStyle.Render(
			keyStyle.Render("j/k")+" navigate  "+keyStyle.Render("Enter/number")+" select  "+keyStyle.Render("Esc")+" cancel",
		))

	case prCommentStepFile:
		title := "File Path"
		if m.commentType == prCommentTypeCode {
			title = "File Path (Code Comment)"
		}
		sb.WriteString(modalTitleStyle.Render(title) + "\n\n")
		sb.WriteString(m.filePath.View() + "\n")
		if len(m.filePaths) > 0 {
			sb.WriteString("\n" + lipgloss.NewStyle().Foreground(colorMuted).Render("Changed files:") + "\n")
			show := m.filePaths
			if len(show) > 8 {
				show = show[:8]
			}
			for _, p := range show {
				sb.WriteString("  " + lipgloss.NewStyle().Foreground(colorMuted).Render(p) + "\n")
			}
		}
		sb.WriteString("\n" + helpStyle.Render(
			keyStyle.Render("Enter")+" next  "+keyStyle.Render("Esc")+" back",
		))

	case prCommentStepLine:
		sb.WriteString(modalTitleStyle.Render("Line Number") + "\n\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render("File: "+m.filePath.Value()) + "\n\n")
		sb.WriteString(m.lineNum.View() + "\n")
		sb.WriteString("\n" + helpStyle.Render(
			keyStyle.Render("Enter")+" next  "+keyStyle.Render("Esc")+" back",
		))

	case prCommentStepThread:
		sb.WriteString(modalTitleStyle.Render("Reply to Thread") + "\n\n")
		for i, t := range m.threads {
			if len(t.Comments) == 0 {
				continue
			}
			preview := strings.ReplaceAll(t.Comments[0].Content, "\n", " ")
			if len(preview) > 60 {
				preview = preview[:57] + "..."
			}
			label := fmt.Sprintf("%d. [%s] %s — %s",
				i+1,
				threadTypeLabel(t),
				t.Comments[0].Author.DisplayName,
				preview,
			)
			if i == m.threadCursor {
				sb.WriteString(selectedItemStyle.Render(label) + "\n")
			} else {
				sb.WriteString(normalItemStyle.Render(label) + "\n")
			}
		}
		sb.WriteString("\n" + helpStyle.Render(
			keyStyle.Render("j/k")+" navigate  "+keyStyle.Render("Enter/number")+" select  "+keyStyle.Render("Esc")+" back",
		))

	case prCommentStepText:
		titles := map[prCommentType]string{
			prCommentTypeGeneral: "General Comment",
			prCommentTypeFile:    "File Comment: " + m.filePath.Value(),
			prCommentTypeCode:    fmt.Sprintf("Code Comment: %s:%s", m.filePath.Value(), m.lineNum.Value()),
			prCommentTypeReply:   "Reply to Thread",
		}
		sb.WriteString(modalTitleStyle.Render(titles[m.commentType]) + "\n\n")
		sb.WriteString(m.text.View())
		sb.WriteString("\n\n" + helpStyle.Render(
			keyStyle.Render("Ctrl+S")+" submit  "+keyStyle.Render("Esc")+" back",
		))
	}

	return modalStyle.Render(sb.String())
}
