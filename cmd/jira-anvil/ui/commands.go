package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pmezard/go-difflib/difflib"
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

type prFetchedMsg struct {
	pr        *api.PullRequest
	build     *api.Build
	fileDiffs []api.FileDiff
	reviewers []api.Reviewer
	err       error
}

type reviewersRefreshedMsg struct {
	reviewers []api.Reviewer
}

type voteDoneMsg struct{}

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

// fetchPRCmd fetches the Azure DevOps pull request for a given Jira issue key,
// along with its diff and latest pipeline build status.
func fetchPRCmd(client *api.AzdoClient, issueKey string) tea.Cmd {
	return func() tea.Msg {
		pr, err := client.GetPRByIssueKey(issueKey)
		if err != nil {
			return prFetchedMsg{err: err}
		}
		if pr == nil {
			return prFetchedMsg{} // no PR found; pr is nil
		}

		// Fetch build, changed files, and reviewers concurrently.
		var (
			build       *api.Build
			files       []api.ChangedFile
			reviewers   []api.Reviewer
			buildErr    error
			filesErr    error
			reviewerErr error
		)
		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			build, buildErr = client.GetLatestBuild(pr.SourceRefName)
		}()
		go func() {
			defer wg.Done()
			files, filesErr = client.GetChangedFiles(pr)
		}()
		go func() {
			defer wg.Done()
			reviewers, reviewerErr = client.GetReviewers(pr)
		}()
		wg.Wait()

		_ = buildErr    // non-fatal: show PR even if build fetch fails
		_ = reviewerErr // non-fatal: show PR even if reviewer fetch fails

		if filesErr != nil {
			return prFetchedMsg{pr: pr, build: build, reviewers: reviewers, err: filesErr}
		}

		// Fetch diffs for up to maxDiffFiles files concurrently.
		const maxDiffFiles = 20
		n := len(files)
		if n > maxDiffFiles {
			n = maxDiffFiles
		}

		fileDiffs := make([]api.FileDiff, len(files))
		var diffWg sync.WaitGroup
		for i := 0; i < n; i++ {
			diffWg.Add(1)
			go func(i int) {
				defer diffWg.Done()
				fileDiffs[i] = computeFileDiff(client, files[i])
			}(i)
		}
		// Files beyond the diff limit: include path/type metadata only.
		for i := n; i < len(files); i++ {
			fileDiffs[i] = api.FileDiff{
				Path:       files[i].Path,
				ChangeType: files[i].ChangeType,
			}
		}
		diffWg.Wait()

		return prFetchedMsg{pr: pr, build: build, fileDiffs: fileDiffs, reviewers: reviewers}
	}
}

// submitVoteCmd submits a vote on a pull request and refreshes the reviewer list.
func submitVoteCmd(client *api.AzdoClient, pr *api.PullRequest, vote int) tea.Cmd {
	return func() tea.Msg {
		reviewerID, err := client.GetCurrentUserID()
		if err != nil {
			return errMsg{fmt.Errorf("fetching user identity: %w", err)}
		}
		if err := client.SubmitVote(pr, vote, reviewerID); err != nil {
			return errMsg{fmt.Errorf("submitting vote: %w", err)}
		}
		reviewers, err := client.GetReviewers(pr)
		if err != nil {
			return voteDoneMsg{}
		}
		return reviewersRefreshedMsg{reviewers: reviewers}
	}
}

// computeFileDiff fetches both blob versions and computes a unified diff.
func computeFileDiff(client *api.AzdoClient, f api.ChangedFile) api.FileDiff {
	fd := api.FileDiff{Path: f.Path, ChangeType: f.ChangeType}

	var baseContent, targetContent string
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		baseContent, _ = client.GetBlob(f.OriginalObjectID)
	}()
	go func() {
		defer wg.Done()
		targetContent, _ = client.GetBlob(f.ObjectID)
	}()
	wg.Wait()

	if isBinaryContent(baseContent) || isBinaryContent(targetContent) {
		fd.Binary = true
		return fd
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(baseContent),
		B:        difflib.SplitLines(targetContent),
		FromFile: "a" + f.Path,
		ToFile:   "b" + f.Path,
		Context:  3,
	}
	diffStr, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return fd
	}
	fd.Hunks = parseDiff(diffStr)
	return fd
}

// isBinaryContent returns true if the string contains null bytes.
func isBinaryContent(s string) bool {
	return strings.ContainsRune(s, 0)
}

// parseDiff parses a unified diff string into DiffHunk/DiffLine slices.
func parseDiff(diffStr string) []api.DiffHunk {
	var hunks []api.DiffHunk
	var current *api.DiffHunk

	for _, line := range strings.Split(diffStr, "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			if current != nil {
				hunks = append(hunks, *current)
			}
			current = &api.DiffHunk{Header: line}
		case strings.HasPrefix(line, "---"), strings.HasPrefix(line, "+++"):
			// skip file header lines
		case current != nil && strings.HasPrefix(line, "+"):
			current.Lines = append(current.Lines, api.DiffLine{Content: line[1:], Type: "added"})
		case current != nil && strings.HasPrefix(line, "-"):
			current.Lines = append(current.Lines, api.DiffLine{Content: line[1:], Type: "deleted"})
		case current != nil && strings.HasPrefix(line, " "):
			current.Lines = append(current.Lines, api.DiffLine{Content: line[1:], Type: "context"})
		}
	}
	if current != nil {
		hunks = append(hunks, *current)
	}
	return hunks
}
