package ui

import (
	"fmt"
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
