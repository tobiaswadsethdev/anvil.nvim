package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/api"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/config"
)

// State represents the current TUI view.
type State int

const (
	StateList            State = iota // issue list
	StateDetail                       // issue detail
	StateTransition                   // transition modal
	StateComment                      // comment modal
	StateAssign                       // assign modal
	StateEdit                         // field editor (external $EDITOR)
	StateVote                         // PR vote modal
	StatePRComment                    // PR comment/reply modal
	StateCreateProject                // project picker for new issue
	StateCreateIssueType              // issue type picker for new issue
)

// createIssueCtx holds the accumulated context during the multi-step issue creation flow.
type createIssueCtx struct {
	projectKey    string
	projectName   string
	issueTypeID   string
	issueTypeName string
	fields        []api.CreateField
}

// Model is the root bubbletea model.
type Model struct {
	cfg         *config.Config
	client      *api.Client
	azdoClient  *api.AzdoClient // nil if Azure DevOps is not configured
	state       State
	filterIndex int
	width       int
	height      int
	err         error
	statusMsg   string

	// Sub-models
	list            ListModel
	detail          DetailModel
	transition      TransitionModel
	comment         CommentModel
	assign          AssignModel
	vote            VoteModel
	prComment       PRCommentModel
	createProject   CreateProjectModel
	createIssueType CreateIssueTypeModel
	createCtx       createIssueCtx
}

// --- Messages ---

type errMsg struct{ err error }
type statusMsg struct{ text string }
type windowSizeMsg struct{ w, h int }

func (e errMsg) Error() string { return e.err.Error() }

// NewModel creates the root model.
func NewModel(cfg *config.Config, client *api.Client, azdoClient *api.AzdoClient) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := Model{
		cfg:        cfg,
		client:     client,
		azdoClient: azdoClient,
		state:      StateList,
	}
	m.list = NewListModel(cfg, client, sp)
	return m
}

// Init kicks off the initial data load.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.list.spinner.Tick,
		m.list.fetchCmd(),
	)
}

// Update handles all messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list = m.list.setSize(msg.Width, msg.Height)
		m.detail = m.detail.setSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case issuesFetchedMsg:
		m.list.loading = false
		m.list.issues = msg.issues
		m.list.total = msg.total
		m.list.buildTable()
		m.statusMsg = fmt.Sprintf("%d issues", msg.total)
		return m, nil

	case issueFetchedMsg:
		hasPRTab := m.azdoClient != nil
		m.detail = NewDetailModel(msg.issue, m.width, m.height, hasPRTab)
		m.state = StateDetail
		if hasPRTab {
			return m, fetchPRCmd(m.azdoClient, msg.issue.Key)
		}
		return m, nil

	case prFetchedMsg:
		if msg.err != nil {
			m.detail.prModel = m.detail.prModel.setError(msg.err)
		} else {
			m.detail.prModel = m.detail.prModel.setData(msg.pr, msg.build, msg.fileDiffs, msg.reviewers, msg.threads)
		}
		return m, nil

	case prThreadsFetchedMsg:
		if msg.err == nil {
			m.detail.prModel = m.detail.prModel.setThreads(msg.threads)
		}
		return m, nil

	case prCommentAddedMsg:
		m.statusMsg = "Comment added"
		m.state = StateDetail
		if m.detail.prModel.pr != nil {
			return m, fetchPRThreadsCmd(m.azdoClient, m.detail.prModel.pr)
		}
		return m, nil

	case reviewersRefreshedMsg:
		m.detail.prModel = m.detail.prModel.setReviewers(msg.reviewers)
		m.statusMsg = "Vote submitted"
		m.state = StateDetail
		return m, nil

	case voteDoneMsg:
		m.statusMsg = "Vote submitted"
		m.state = StateDetail
		return m, nil

	case transitionsDoneMsg:
		m.statusMsg = "Transitioned successfully"
		m.state = StateDetail
		return m.reloadDetail()

	case commentDoneMsg:
		m.statusMsg = "Comment added"
		m.state = StateDetail
		return m.reloadDetail()

	case assignDoneMsg:
		m.statusMsg = "Assigned successfully"
		m.state = StateDetail
		return m.reloadDetail()

	case editDoneMsg:
		m.statusMsg = "Updated successfully"
		m.state = StateDetail
		return m.reloadDetail()

	case transitionsLoadedMsg:
		m.transition = NewTransitionModel(msg.transitions, msg.issueKey)
		m.state = StateTransition
		return m, nil

	case assignableUsersLoadedMsg:
		m.assign = NewAssignModel(msg.users, msg.issueKey)
		m.state = StateAssign
		return m, nil

	case execEditorMsg:
		return m, MakeExecEditorCmd(msg)

	case projectsLoadedMsg:
		m.list.loading = false
		m.createProject = NewCreateProjectModel(msg.projects)
		m.state = StateCreateProject
		return m, nil

	case issueTypesForCreateLoadedMsg:
		m.createCtx.projectKey = msg.projectKey
		m.createCtx.projectName = msg.projectName
		m.createIssueType = NewCreateIssueTypeModel(msg.issueTypes)
		m.state = StateCreateIssueType
		return m, nil

	case createMetaLoadedMsg:
		m.createCtx = msg.ctx
		return m, generateAndOpenCreateEditorCmd(m.client, m.createCtx)

	case execCreateEditorMsg:
		return m, MakeExecCreateEditorCmd(msg)

	case issueCreatedMsg:
		m.statusMsg = "Created " + msg.key
		m.state = StateList
		m.list.loading = true
		return m, tea.Batch(m.list.spinner.Tick, m.list.fetchCmd())

	case errMsg:
		m.err = msg.err
		m.list.loading = false
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.list.spinner, cmd = m.list.spinner.Update(msg)
		return m, cmd
	}

	// Delegate to sub-models
	return m.updateSubModel(msg)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global quit
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.state {
	case StateList:
		return m.handleListKey(msg)
	case StateDetail:
		return m.handleDetailKey(msg)
	case StateTransition:
		return m.handleTransitionKey(msg)
	case StateComment:
		return m.handleCommentKey(msg)
	case StateAssign:
		return m.handleAssignKey(msg)
	case StateVote:
		return m.handleVoteKey(msg)
	case StatePRComment:
		return m.handlePRCommentKey(msg)
	case StateCreateProject:
		return m.handleCreateProjectKey(msg)
	case StateCreateIssueType:
		return m.handleCreateIssueTypeKey(msg)
	}
	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "]":
		m.filterIndex = (m.filterIndex + 1) % len(m.cfg.Filters)
		m.list.filterIndex = m.filterIndex
		m.list.loading = true
		return m, tea.Batch(m.list.spinner.Tick, m.list.fetchCmd())
	case "[":
		m.filterIndex = (m.filterIndex - 1 + len(m.cfg.Filters)) % len(m.cfg.Filters)
		m.list.filterIndex = m.filterIndex
		m.list.loading = true
		return m, tea.Batch(m.list.spinner.Tick, m.list.fetchCmd())
	case "r":
		m.list.loading = true
		return m, tea.Batch(m.list.spinner.Tick, m.list.fetchCmd())
	case "n":
		m.list.loading = true
		return m, tea.Batch(m.list.spinner.Tick, loadProjectsCmd(m.client))
	case "o":
		if issue := m.list.selectedIssue(); issue != nil {
			openBrowser(m.cfg.Jira.URL + "/browse/" + issue.Key)
		}
		return m, nil
	case "enter":
		if issue := m.list.selectedIssue(); issue != nil {
			return m, fetchIssueCmd(m.client, issue.Key)
		}
		return m, nil
	case "?":
		m.statusMsg = listHelp
		return m, nil
	}

	// Pass to list table for navigation
	var cmd tea.Cmd
	m.list, cmd = m.list.update(msg)
	return m, cmd
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.state = StateList
		return m, nil
	case "]":
		if m.detail.hasPRTab {
			m.detail.tabIndex = (m.detail.tabIndex + 1) % 2
		}
		return m, nil
	case "[":
		if m.detail.hasPRTab {
			m.detail.tabIndex = (m.detail.tabIndex - 1 + 2) % 2
		}
		return m, nil
	case "r":
		return m.reloadDetail()
	case "o":
		openBrowser(m.cfg.Jira.URL + "/browse/" + m.detail.issue.Key)
		return m, nil
	case "t":
		if m.detail.tabIndex == 0 {
			return m, loadTransitionsCmd(m.client, m.detail.issue.Key)
		}
		return m, nil
	case "c":
		// PR tab: open PR comment modal; Jira tab: open Jira comment modal
		if m.detail.hasPRTab && m.detail.tabIndex == 1 && m.detail.prModel.pr != nil {
			filePaths := make([]string, 0, len(m.detail.prModel.fileDiffs))
			for _, fd := range m.detail.prModel.fileDiffs {
				filePaths = append(filePaths, fd.Path)
			}
			m.prComment = NewPRCommentModel(m.detail.prModel.threads, filePaths)
			m.state = StatePRComment
			return m, m.prComment.Init()
		}
		m.comment = NewCommentModel(m.detail.issue.Key)
		m.state = StateComment
		return m, m.comment.Init()
	case "a":
		if m.detail.tabIndex == 0 {
			return m, loadAssignableUsersCmd(m.client, m.detail.issue.Key)
		}
		return m, nil
	case "e":
		if m.detail.tabIndex == 0 {
			return m, startEditCmd(m.detail.issue, m.client)
		}
		return m, nil
	case "v":
		if m.detail.hasPRTab && m.detail.tabIndex == 1 && m.detail.prModel.pr != nil {
			m.vote = NewVoteModel(m.detail.prModel.pr)
			m.state = StateVote
			return m, nil
		}
		return m, nil
	case "y":
		if m.detail.hasPRTab && m.detail.tabIndex == 1 && m.detail.prModel.pr != nil {
			url := m.azdoClient.PRWebURL(m.detail.prModel.pr)
			if copyToClipboard(url) {
				m.statusMsg = "PR link copied to clipboard"
			} else {
				m.statusMsg = "Failed to copy: install xclip, wl-copy, or pbcopy"
			}
		}
		return m, nil
	case "?":
		m.statusMsg = detailHelp
		return m, nil
	}

	// Pass to viewport for scrolling
	var cmd tea.Cmd
	m.detail, cmd = m.detail.update(msg)
	return m, cmd
}

func (m Model) handleTransitionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.state = StateDetail
		return m, nil
	}
	var cmd tea.Cmd
	var done bool
	m.transition, cmd, done = m.transition.update(msg, m.client)
	if done {
		m.state = StateDetail
		return m.reloadDetail()
	}
	return m, cmd
}

func (m Model) handleCommentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = StateDetail
		return m, nil
	}
	var cmd tea.Cmd
	var done bool
	m.comment, cmd, done = m.comment.update(msg, m.client)
	if done {
		m.state = StateDetail
	}
	return m, cmd
}

func (m Model) handleAssignKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.state = StateDetail
		return m, nil
	}
	var cmd tea.Cmd
	var done bool
	m.assign, cmd, done = m.assign.update(msg, m.client)
	if done {
		m.state = StateDetail
		return m.reloadDetail()
	}
	return m, cmd
}

func (m Model) handleVoteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.state = StateDetail
		return m, nil
	}
	var selected *voteOption
	var done bool
	m.vote, selected, done = m.vote.update(msg)
	if done {
		m.state = StateDetail
		if selected != nil {
			return m, submitVoteCmd(m.azdoClient, m.vote.pr, selected.value)
		}
	}
	return m, nil
}

func (m Model) handlePRCommentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var result *PRCommentResult
	var done bool
	m.prComment, result, done = m.prComment.update(msg)
	if done {
		m.state = StateDetail
		if result != nil && m.detail.prModel.pr != nil {
			pr := m.detail.prModel.pr
			if result.ThreadID != 0 {
				return m, replyToPRThreadCmd(m.azdoClient, pr, result.ThreadID, result.ParentID, result.Content)
			}
			ctx := buildPRThreadContext(result)
			return m, addPRThreadCmd(m.azdoClient, pr, result.Content, ctx)
		}
	}
	return m, nil
}

// buildPRThreadContext constructs the Azure DevOps thread context from a comment result.
// Returns nil for general comments (no file/code context).
func buildPRThreadContext(r *PRCommentResult) *api.PRThreadContext {
	if r.FilePath == "" {
		return nil
	}
	ctx := &api.PRThreadContext{FilePath: r.FilePath}
	if r.Line > 0 {
		pos := &api.PRFilePosition{Line: r.Line, Offset: 1}
		ctx.RightFileStart = pos
		ctx.RightFileEnd = pos
	}
	return ctx
}

func (m Model) updateSubModel(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case StateList:
		var cmd tea.Cmd
		m.list, cmd = m.list.update(msg)
		return m, cmd
	case StateDetail:
		var cmd tea.Cmd
		m.detail, cmd = m.detail.update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleCreateProjectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.state = StateList
		m.list.loading = false
		return m, nil
	}
	var cmd tea.Cmd
	var selected *api.Project
	m.createProject, selected, cmd = m.createProject.update(msg)
	if selected != nil {
		return m, loadIssueTypesForCreateCmd(m.client, selected.Key, selected.Name)
	}
	return m, cmd
}

func (m Model) handleCreateIssueTypeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.state = StateCreateProject
		return m, nil
	}
	var cmd tea.Cmd
	var selected *api.CreateIssueType
	m.createIssueType, selected, cmd = m.createIssueType.update(msg)
	if selected != nil {
		ctx := m.createCtx
		ctx.issueTypeID = selected.ID
		ctx.issueTypeName = selected.Name
		return m, loadCreateMetaCmd(m.client, ctx)
	}
	return m, cmd
}

func (m Model) reloadDetail() (tea.Model, tea.Cmd) {
	if m.detail.issue == nil || m.detail.issue.Key == "" {
		m.state = StateList
		return m, nil
	}
	return m, fetchIssueCmd(m.client, m.detail.issue.Key)
}

// View renders the current state.
func (m Model) View() string {
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error() + "\n\nPress q to quit.")
	}

	switch m.state {
	case StateList:
		return m.list.view()
	case StateDetail:
		return m.detail.view()
	case StateTransition:
		return m.renderOverlay(m.list.view(), m.transition.view())
	case StateComment:
		return m.renderOverlay(m.detail.view(), m.comment.view())
	case StateAssign:
		return m.renderOverlay(m.detail.view(), m.assign.view())
	case StateVote:
		return m.renderOverlay(m.detail.view(), m.vote.view())
	case StatePRComment:
		return m.renderOverlay(m.detail.view(), m.prComment.view())
	case StateCreateProject:
		return m.renderOverlay(m.list.view(), m.createProject.view())
	case StateCreateIssueType:
		return m.renderOverlay(m.list.view(), m.createIssueType.view())
	}
	return ""
}

func (m Model) renderOverlay(base, modal string) string {
	return lipgloss.JoinVertical(lipgloss.Left, base, modal)
}

// Help strings
const listHelp = "↑/↓: navigate  Enter: open  [/]: cycle filter  r: refresh  n: new issue  o: browser  q: quit"
const detailHelp = "↑/↓: scroll  [/]: tab  t: transition  c: comment/PR comment  a: assign  e: edit  v: vote (PR tab)  y: copy PR link (PR tab)  o: browser  q: back"
