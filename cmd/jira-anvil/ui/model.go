package ui

import (
	"fmt"
	"strings"

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
	StateCreatePR                     // create pull request modal
	StateHelp                         // help modal
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
	createPR        CreatePRModel
	help            HelpModel
	helpReturnState State
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
		hasPR := m.azdoClient != nil
		m.detail = NewDetailModel(msg.issue, m.width, m.height, hasPR)
		m.state = StateDetail
		if hasPR {
			return m, fetchPRCmd(m.azdoClient, msg.issue.Key)
		}
		return m, nil

	case prFetchedMsg:
		if msg.err != nil {
			m.detail.prModel = m.detail.prModel.setError(msg.err)
		} else {
			m.detail.prModel = m.detail.prModel.setData(msg.pr, msg.build, msg.fileDiffs, msg.reviewers, msg.threads)
		}
		if m.detail.hasPR {
			m.detail.refreshPRInfoViewport()
			m.detail.refreshCenterViewport()
			m.detail.refreshRightViewport()
		}
		return m, nil

	case prThreadsFetchedMsg:
		if msg.err == nil {
			m.detail.prModel = m.detail.prModel.setThreads(msg.threads)
			if m.detail.hasPR {
				m.detail.refreshPRInfoViewport()
				m.detail.refreshRightViewport()
			}
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
		if m.detail.hasPR {
			m.detail.refreshPRInfoViewport()
		}
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

	case branchesLoadedMsg:
		m.createPR = NewCreatePRModel(msg.branches, msg.repoName, msg.issueKey, msg.issueSummary, msg.currentBranch)
		m.state = StateCreatePR
		return m, m.createPR.Init()

	case prCreatedMsg:
		m.statusMsg = fmt.Sprintf("PR #%d created", msg.pr.PullRequestID)
		m.state = StateDetail
		return m, fetchPRCmd(m.azdoClient, m.detail.issue.Key)

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
	case StateCreatePR:
		return m.handleCreatePRKey(msg)
	case StateHelp:
		return m.handleHelpKey(msg)
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
		m.help = NewHelpModel("Issue List Help", listHelpText)
		m.helpReturnState = StateList
		m.state = StateHelp
		return m, nil
	}

	// Pass to list table for navigation
	var cmd tea.Cmd
	m.list, cmd = m.list.update(msg)
	return m, cmd
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := m.detail.numPanels()

	switch msg.String() {
	case "q", "esc":
		m.state = StateList
		return m, nil

	// Panel navigation
	case "tab":
		m.detail.focusedPanel = (m.detail.focusedPanel + 1) % n
		return m, nil
	case "shift+tab":
		m.detail.focusedPanel = (m.detail.focusedPanel - 1 + n) % n
		return m, nil
	case "1":
		m.detail.focusedPanel = 0
		return m, nil
	case "2":
		if n >= 2 {
			m.detail.focusedPanel = 1
		}
		return m, nil
	case "3":
		if n >= 3 {
			m.detail.focusedPanel = 2
		}
		return m, nil
	case "4":
		if n >= 4 {
			m.detail.focusedPanel = 3
		}
		return m, nil

	// Tab switching within focused panel
	case "]":
		m.detail = m.detailNextTab(1)
		return m, nil
	case "[":
		m.detail = m.detailNextTab(-1)
		return m, nil

	case "r":
		return m.reloadDetail()
	case "o":
		openBrowser(m.cfg.Jira.URL + "/browse/" + m.detail.issue.Key)
		return m, nil

	// Jira actions (always available)
	case "t":
		return m, loadTransitionsCmd(m.client, m.detail.issue.Key)
	case "a":
		return m, loadAssignableUsersCmd(m.client, m.detail.issue.Key)
	case "e":
		return m, startEditCmd(m.detail.issue, m.client)
	case "c":
		// If PR panel is focused and PR data is loaded, open PR comment modal
		isPRContext := m.detail.focusedPanel == panelPRInfo || m.detail.focusedPanel == panelCenter ||
			(m.detail.focusedPanel == panelRight && m.detail.rightTabIndex == 0)
		if m.detail.hasPR && m.detail.prModel.pr != nil && isPRContext {
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

	// PR-only actions
	case "v":
		if m.detail.hasPR && m.detail.prModel.pr != nil {
			m.vote = NewVoteModel(m.detail.prModel.pr)
			m.state = StateVote
			return m, nil
		}
		return m, nil
	case "y":
		if m.detail.hasPR && m.detail.prModel.pr != nil {
			url := m.azdoClient.PRWebURL(m.detail.prModel.pr)
			if copyToClipboard(url) {
				m.statusMsg = "PR link copied to clipboard"
			} else {
				m.statusMsg = "Failed to copy: install xclip, wl-copy, or pbcopy"
			}
		}
		return m, nil

	case "p":
		if m.detail.hasPR && m.detail.prModel.notFound {
			return m, loadBranchesCmd(m.azdoClient, m.detail.issue.Key, m.detail.issue.Fields.Summary)
		}
		return m, nil

	case "?":
		m.help = NewHelpModel("Issue Detail Help", detailHelpText(m.detail.hasPR, m.detail.prModel.pr != nil, m.detail.prModel.notFound))
		m.helpReturnState = StateDetail
		m.state = StateHelp
		return m, nil
	}

	// Pass scroll keys to the focused panel's viewport
	var cmd tea.Cmd
	m.detail, cmd = m.detail.update(msg)
	return m, cmd
}

// detailNextTab advances (or reverses) the tab within the currently focused panel.
func (m Model) detailNextTab(dir int) DetailModel {
	d := m.detail
	if !d.hasPR {
		if d.focusedPanel == panelDescNoPR {
			tabs := 2 // Description | Comments
			d.descTabIndex = (d.descTabIndex + dir + tabs) % tabs
			d.refreshNoPRDescViewport()
		}
		return d
	}

	switch d.focusedPanel {
	case panelCenter:
		tabs := 3 // Files | Diff | Jira Description
		d.centerTabIndex = (d.centerTabIndex + dir + tabs) % tabs
		d.refreshCenterViewport()
	case panelRight:
		tabs := 3 // PR Comments | Jira Comments | Jira History
		d.rightTabIndex = (d.rightTabIndex + dir + tabs) % tabs
		d.refreshRightViewport()
	}
	return d
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
		return m, cmd
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

func (m Model) handleCreatePRKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var result *CreatePRResult
	var done bool
	m.createPR, result, done = m.createPR.update(msg)
	if done {
		m.state = StateDetail
		if result != nil {
			return m, createPRCmd(m.azdoClient, result.RepoName, result.Title, result.Description, result.Source, result.Target)
		}
	}
	return m, nil
}

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var done bool
	m.help, done = m.help.update(msg)
	if done {
		m.state = m.helpReturnState
	}
	return m, nil
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
	case StateCreatePR:
		return m.renderOverlay(m.detail.view(), m.createPR.view())
	case StateHelp:
		base := m.list.view()
		if m.helpReturnState == StateDetail {
			base = m.detail.view()
		}
		return m.renderOverlay(base, m.help.view())
	}
	return ""
}

func (m Model) renderOverlay(base, modal string) string {
	if m.width < 1 || m.height < 1 {
		return lipgloss.JoinVertical(lipgloss.Left, base, modal)
	}

	baseNorm := normalizeBlock(base, m.width, m.height)

	modalW := lipgloss.Width(modal)
	if modalW < 1 {
		modalW = maxInt(1, m.width*70/100)
	}
	if modalW > m.width {
		modalW = m.width
	}
	modalH := lipgloss.Height(modal)
	if modalH < 1 {
		modalH = 1
	}
	if modalH > m.height {
		modalH = m.height
	}

	modalX := (m.width - modalW) / 2
	if modalX < 0 {
		modalX = 0
	}
	modalY := (m.height - modalH) / 2
	if modalY < 0 {
		modalY = 0
	}

	modalBlock := lipgloss.NewStyle().Width(modalW).Render(modal)
	modalNorm := normalizeBlock(modalBlock, modalW, modalH)

	blocks := []positionedBlock{}
	if modalY > 0 {
		blocks = append(blocks, positionedBlock{
			rect:  Rect{X: 0, Y: 0, W: m.width, H: modalY},
			lines: baseNorm[:modalY],
		})
	}
	bottomY := modalY + modalH
	if bottomY < m.height {
		blocks = append(blocks, positionedBlock{
			rect:  Rect{X: 0, Y: bottomY, W: m.width, H: m.height - bottomY},
			lines: baseNorm[bottomY:],
		})
	}
	blocks = append(blocks, positionedBlock{
		rect:  Rect{X: modalX, Y: modalY, W: modalW, H: modalH},
		lines: modalNorm,
	})

	return composeRectGrid(m.width, m.height, blocks)
}

// Help strings
const listHelp = "↑/↓: navigate  Enter: open  [/]: cycle filter  r: refresh  n: new issue  o: browser  q: quit"
const detailHelp = "Tab/S-Tab: panel  1-4: jump  [/]: tab  ↑/↓: scroll  t: transition  c: comment (Jira/PR)  a: assign  e: edit  v: vote (PR)  y: copy PR link  p: create PR (when no PR)  o: browser  q: back"

const listHelpText = "Navigation:\n  ↑/↓        Move selection\n  Enter      Open selected issue\n\nActions:\n  [/]        Cycle filter\n  r          Refresh issues\n  n          Create new issue\n  o          Open issue in browser\n\nOther:\n  ?          Show this help\n  q          Quit"

func detailHelpText(hasPR bool, hasLoadedPR bool, prNotFound bool) string {
	lines := []string{
		"Navigation:",
		"  Tab/S-Tab  Switch focused panel",
		"  1-4        Jump to panel",
		"  [/]        Switch tabs in focused panel",
		"  ↑/↓        Scroll focused panel",
		"  r          Refresh issue/PR data",
		"",
		"Actions:",
		"  t          Transition issue",
		"  c          Add comment",
		"  a          Assign issue",
		"  e          Edit issue",
	}

	if hasPR && hasLoadedPR {
		lines = append(lines,
			"  v          Vote on pull request",
			"  y          Copy PR link",
		)
	}
	if hasPR && prNotFound {
		lines = append(lines, "  p          Create pull request")
	}

	lines = append(lines,
		"",
		"Other:",
		"  o          Open Jira issue in browser",
		"  ?          Show this help",
		"  q / Esc    Back to issue list",
	)

	return strings.Join(lines, "\n")
}
