package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/api"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/ui/adf"
)

// --- Message types for async operations ---

type issuesFetchedMsg struct {
	issues []api.Issue
	total  int
}

type issueFetchedMsg struct {
	issue *api.Issue
}

type transitionsDoneMsg struct{}
type commentDoneMsg struct{}
type assignDoneMsg struct{}
type editDoneMsg struct{}

type transitionsLoadedMsg struct {
	transitions []api.Transition
	issueKey    string
}

type assignableUsersLoadedMsg struct {
	users    []api.User
	issueKey string
}

// --- Commands ---

func fetchIssueCmd(client *api.Client, key string) tea.Cmd {
	return func() tea.Msg {
		issue, err := client.GetIssue(key)
		if err != nil {
			return errMsg{err}
		}
		return issueFetchedMsg{issue}
	}
}

func loadTransitionsCmd(client *api.Client, issueKey string) tea.Cmd {
	return func() tea.Msg {
		transitions, err := client.GetTransitions(issueKey)
		if err != nil {
			return errMsg{err}
		}
		return transitionsLoadedMsg{transitions, issueKey}
	}
}

func loadAssignableUsersCmd(client *api.Client, issueKey string) tea.Cmd {
	return func() tea.Msg {
		users, err := client.GetAssignableUsers(issueKey)
		if err != nil {
			return errMsg{err}
		}
		return assignableUsersLoadedMsg{users, issueKey}
	}
}

func doTransitionCmd(client *api.Client, issueKey, transitionID string) tea.Cmd {
	return func() tea.Msg {
		if err := client.DoTransition(issueKey, transitionID); err != nil {
			return errMsg{err}
		}
		return transitionsDoneMsg{}
	}
}

func addCommentCmd(client *api.Client, issueKey string, body json.RawMessage) tea.Cmd {
	return func() tea.Msg {
		if err := client.AddComment(issueKey, body); err != nil {
			return errMsg{err}
		}
		return commentDoneMsg{}
	}
}

func assignIssueCmd(client *api.Client, issueKey, accountID string) tea.Cmd {
	return func() tea.Msg {
		if err := client.AssignIssue(issueKey, accountID); err != nil {
			return errMsg{err}
		}
		return assignDoneMsg{}
	}
}

// startEditCmd opens the field editor.
// If the issue has multiple ADF fields, it opens the description.
// The editor is launched via tea.ExecProcess.
func startEditCmd(issue *api.Issue, client *api.Client) tea.Cmd {
	return func() tea.Msg {
		// Convert description ADF to Markdown
		md := adf.ToMarkdown(issue.Fields.Description)
		if md == "" {
			md = ""
		}

		// Write to temp file
		tmpFile, err := os.CreateTemp("", "anvil-edit-*.md")
		if err != nil {
			return errMsg{fmt.Errorf("creating temp file: %w", err)}
		}
		tmpPath := tmpFile.Name()
		if _, err := tmpFile.WriteString(md); err != nil {
			tmpFile.Close()
			return errMsg{fmt.Errorf("writing temp file: %w", err)}
		}
		tmpFile.Close()

		// Launch editor
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nvim"
		}

		// Return a tea.ExecProcess command
		return execEditorMsg{editor: editor, path: tmpPath, issueKey: issue.Key, client: client}
	}
}

// execEditorMsg triggers the ExecProcess path in Update.
type execEditorMsg struct {
	editor   string
	path     string
	issueKey string
	client   *api.Client
}

// MakeExecEditorCmd creates the actual tea.ExecProcess command when we have the msg.
func MakeExecEditorCmd(msg execEditorMsg) tea.Cmd {
	cmd := exec.Command(msg.editor, msg.path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			os.Remove(msg.path)
			return errMsg{fmt.Errorf("editor: %w", err)}
		}

		// Read edited content
		data, readErr := os.ReadFile(msg.path)
		os.Remove(msg.path)
		if readErr != nil {
			return errMsg{fmt.Errorf("reading edited file: %w", readErr)}
		}

		// Convert Markdown back to ADF
		adfDoc := adf.FromMarkdown(string(data))

		// Update the issue description
		fields := map[string]interface{}{
			"description": json.RawMessage(adfDoc),
		}
		if updateErr := msg.client.UpdateIssue(msg.issueKey, fields); updateErr != nil {
			return errMsg{fmt.Errorf("updating issue: %w", updateErr)}
		}
		return editDoneMsg{}
	})
}

// openBrowser opens a URL in the system default browser.
func openBrowser(url string) {
	cmd := exec.Command("xdg-open", url)
	cmd.Start()
}
