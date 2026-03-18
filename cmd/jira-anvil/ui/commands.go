package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
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
	threads   []api.PRCommentThread
	err       error
}

type reviewersRefreshedMsg struct {
	reviewers []api.Reviewer
}

type voteDoneMsg struct{}

type prThreadsFetchedMsg struct {
	threads []api.PRCommentThread
	err     error
}

type prCommentAddedMsg struct{}

type projectsLoadedMsg struct {
	projects []api.Project
}

type issueTypesForCreateLoadedMsg struct {
	issueTypes  []api.CreateIssueType
	projectKey  string
	projectName string
}

type createMetaLoadedMsg struct {
	ctx createIssueCtx
}

type issueCreatedMsg struct {
	key string
}

type execCreateEditorMsg struct {
	path   string
	ctx    createIssueCtx
	client *api.Client
}

type branchesLoadedMsg struct {
	branches      []string
	repoName      string
	issueKey      string
	issueSummary  string
	currentBranch string
}

type prCreatedMsg struct {
	pr *api.PullRequest
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

// copyToClipboard writes text to the system clipboard.
// Tries wl-copy (Wayland), xclip (X11), and pbcopy (macOS) in order.
// Returns true if a clipboard tool was found and succeeded.
func copyToClipboard(text string) bool {
	candidates := [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"pbcopy"},
	}
	for _, args := range candidates {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return true
		}
	}
	return false
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

		// Fetch build, changed files, reviewers, and threads concurrently.
		var (
			build       *api.Build
			files       []api.ChangedFile
			reviewers   []api.Reviewer
			threads     []api.PRCommentThread
			buildErr    error
			filesErr    error
			reviewerErr error
			threadsErr  error
		)
		var wg sync.WaitGroup
		wg.Add(4)
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
		go func() {
			defer wg.Done()
			threads, threadsErr = client.GetPRThreads(pr)
		}()
		wg.Wait()

		_ = buildErr    // non-fatal: show PR even if build fetch fails
		_ = reviewerErr // non-fatal: show PR even if reviewer fetch fails
		_ = threadsErr  // non-fatal: show PR even if thread fetch fails

		if filesErr != nil {
			return prFetchedMsg{pr: pr, build: build, reviewers: reviewers, threads: threads, err: filesErr}
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

		return prFetchedMsg{pr: pr, build: build, fileDiffs: fileDiffs, reviewers: reviewers, threads: threads}
	}
}

// fetchPRThreadsCmd fetches only the comment threads for a pull request (used to refresh after adding a comment).
func fetchPRThreadsCmd(client *api.AzdoClient, pr *api.PullRequest) tea.Cmd {
	return func() tea.Msg {
		threads, err := client.GetPRThreads(pr)
		return prThreadsFetchedMsg{threads: threads, err: err}
	}
}

// addPRThreadCmd creates a new PR comment thread.
func addPRThreadCmd(client *api.AzdoClient, pr *api.PullRequest, content string, ctx *api.PRThreadContext) tea.Cmd {
	return func() tea.Msg {
		if err := client.AddPRThread(pr, content, ctx); err != nil {
			return errMsg{err}
		}
		return prCommentAddedMsg{}
	}
}

// replyToPRThreadCmd adds a reply to an existing PR comment thread.
func replyToPRThreadCmd(client *api.AzdoClient, pr *api.PullRequest, threadID, parentCommentID int, content string) tea.Cmd {
	return func() tea.Msg {
		if err := client.ReplyToPRThread(pr, threadID, parentCommentID, content); err != nil {
			return errMsg{err}
		}
		return prCommentAddedMsg{}
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
		baseContent, _ = client.GetBlob(f.OriginalObjectID, f.RepoName)
	}()
	go func() {
		defer wg.Done()
		targetContent, _ = client.GetBlob(f.ObjectID, f.RepoName)
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

// loadProjectsCmd fetches all Jira projects the user has access to.
func loadProjectsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		projects, err := client.GetProjects()
		if err != nil {
			return errMsg{err}
		}
		return projectsLoadedMsg{projects}
	}
}

// loadIssueTypesForCreateCmd fetches issue types available for creating issues in a project.
func loadIssueTypesForCreateCmd(client *api.Client, projectKey, projectName string) tea.Cmd {
	return func() tea.Msg {
		types, err := client.GetCreateMetaIssueTypes(projectKey)
		if err != nil {
			return errMsg{err}
		}
		return issueTypesForCreateLoadedMsg{types, projectKey, projectName}
	}
}

// loadCreateMetaCmd fetches all available fields for creating an issue of the given type.
func loadCreateMetaCmd(client *api.Client, ctx createIssueCtx) tea.Cmd {
	return func() tea.Msg {
		fields, err := client.GetCreateMetaFields(ctx.projectKey, ctx.issueTypeID)
		if err != nil {
			return errMsg{err}
		}
		ctx.fields = fields
		return createMetaLoadedMsg{ctx}
	}
}

// generateAndOpenCreateEditorCmd generates a YAML issue template and signals the editor to open.
func generateAndOpenCreateEditorCmd(client *api.Client, ctx createIssueCtx) tea.Cmd {
	return func() tea.Msg {
		template := generateIssueTemplate(ctx)
		tmpFile, err := os.CreateTemp("", "anvil-create-*.yaml")
		if err != nil {
			return errMsg{fmt.Errorf("creating temp file: %w", err)}
		}
		tmpPath := tmpFile.Name()
		if _, err := tmpFile.WriteString(template); err != nil {
			tmpFile.Close()
			return errMsg{fmt.Errorf("writing temp file: %w", err)}
		}
		tmpFile.Close()
		return execCreateEditorMsg{path: tmpPath, ctx: ctx, client: client}
	}
}

// MakeExecCreateEditorCmd opens the editor for issue creation and submits after save.
func MakeExecCreateEditorCmd(msg execCreateEditorMsg) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}
	cmd := exec.Command(editor, msg.path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.Remove(msg.path)
		if err != nil {
			return errMsg{fmt.Errorf("editor: %w", err)}
		}

		data, readErr := os.ReadFile(msg.path)
		if readErr != nil {
			return errMsg{fmt.Errorf("reading file: %w", readErr)}
		}

		fields, parseErr := parseIssueTemplate(string(data), msg.ctx, msg.client)
		if parseErr != nil {
			return errMsg{parseErr}
		}

		fields["project"] = map[string]string{"key": msg.ctx.projectKey}
		fields["issuetype"] = map[string]string{"id": msg.ctx.issueTypeID}

		key, createErr := msg.client.CreateIssue(fields)
		if createErr != nil {
			return errMsg{fmt.Errorf("creating issue: %w", createErr)}
		}
		return issueCreatedMsg{key}
	})
}

// systemFieldsToSkip are fields that are auto-set by Jira or already handled elsewhere.
var systemFieldsToSkip = map[string]bool{
	"project":                       true,
	"issuetype":                     true,
	"status":                        true,
	"created":                       true,
	"updated":                       true,
	"creator":                       true,
	"reporter":                      true,
	"lastViewed":                    true,
	"watches":                       true,
	"votes":                         true,
	"worklog":                       true,
	"timetracking":                  true,
	"attachment":                    true,
	"subtasks":                      true,
	"issuelinks":                    true,
	"comment":                       true,
	"thumbnail":                     true,
	"aggregateprogress":             true,
	"progress":                      true,
	"timespent":                     true,
	"aggregatetimespent":            true,
	"timeestimate":                  true,
	"aggregatetimeestimate":         true,
	"aggregatetimeoriginalestimate": true,
	"timeoriginalestimate":          true,
	"workratio":                     true,
	"resolutiondate":                true,
	"resolution":                    true,
}

// generateIssueTemplate produces an annotated YAML template for all fields available
// when creating an issue of the type described by ctx.
func generateIssueTemplate(ctx createIssueCtx) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Create Jira Issue: %s (%s) - %s\n", ctx.projectName, ctx.projectKey, ctx.issueTypeName))
	sb.WriteString("# Fill in the values below. Lines starting with # are comments and are ignored.\n")
	sb.WriteString("# Required fields are marked [REQUIRED]. Save and close to submit.\n")
	sb.WriteString("# Leave optional fields as empty string \"\" to skip them.\n\n")

	// Separate required from optional fields, and standard from custom
	var required, standard, custom []api.CreateField
	for _, f := range ctx.fields {
		if systemFieldsToSkip[f.FieldID] {
			continue
		}
		if f.Required {
			required = append(required, f)
		} else if strings.HasPrefix(f.FieldID, "customfield_") {
			custom = append(custom, f)
		} else {
			standard = append(standard, f)
		}
	}

	// Write required fields first
	for _, f := range required {
		writeFieldTemplate(&sb, f, true)
	}

	// Write optional standard fields
	if len(standard) > 0 {
		sb.WriteString("\n# ── Optional Fields ──────────────────────────────────\n\n")
		for _, f := range standard {
			writeFieldTemplate(&sb, f, false)
		}
	}

	// Write custom fields
	if len(custom) > 0 {
		sb.WriteString("\n# ── Custom Fields ────────────────────────────────────\n\n")
		for _, f := range custom {
			writeFieldTemplate(&sb, f, false)
		}
	}

	return sb.String()
}

// isADFField reports whether a field should be serialised as ADF (Atlassian
// Document Format) when submitted to the Jira Cloud REST API v3.
// The description field may be returned with Schema.Type == "string" by the
// create-meta endpoint even though it requires ADF.  Some custom richtext /
// textarea fields also carry non-"doc" schema types but still need ADF.
func isADFField(f api.CreateField) bool {
	if f.Schema.Type == "doc" {
		return true
	}
	if f.FieldID == "description" {
		return true
	}
	// Known Atlassian custom field types that accept ADF.
	switch f.Schema.Custom {
	case "com.atlassian.jira.plugin.system.customfieldtypes:richtext",
		"com.atlassian.jira.plugin.system.customfieldtypes:textarea":
		return true
	}
	return false
}

// writeFieldTemplate writes the template entry for a single field.
func writeFieldTemplate(sb *strings.Builder, f api.CreateField, required bool) {
	// Comment line with field name and info
	label := f.Name
	if required {
		label = "[REQUIRED] " + label
	}

	schemaHint := fieldSchemaHint(f)
	if schemaHint != "" {
		sb.WriteString(fmt.Sprintf("# %s (%s)\n", label, schemaHint))
	} else {
		sb.WriteString(fmt.Sprintf("# %s\n", label))
	}

	// List allowed values as a comment if available
	if len(f.AllowedValues) > 0 && len(f.AllowedValues) <= 20 {
		opts := make([]string, 0, len(f.AllowedValues))
		for _, av := range f.AllowedValues {
			if av.Name != "" {
				opts = append(opts, av.Name)
			} else if av.Value != "" {
				opts = append(opts, av.Value)
			}
		}
		if len(opts) > 0 {
			sb.WriteString(fmt.Sprintf("# Options: %s\n", strings.Join(opts, " | ")))
		}
	}

	// Field key and default value
	if isADFField(f) {
		sb.WriteString(fmt.Sprintf("%s: |\n  \n\n", f.FieldID))
	} else {
		sb.WriteString(fmt.Sprintf("%s: \"\"\n\n", f.FieldID))
	}
}

// fieldSchemaHint returns a human-readable type hint for a field schema.
func fieldSchemaHint(f api.CreateField) string {
	t := f.Schema.Type
	switch t {
	case "string":
		return "text"
	case "number":
		return "number"
	case "date":
		return "date: YYYY-MM-DD"
	case "datetime":
		return "datetime: YYYY-MM-DDTHH:MM:SS.000+0000"
	case "user":
		return "email address"
	case "priority":
		return "priority name"
	case "option":
		return "option value"
	case "array":
		switch f.Schema.Items {
		case "string":
			return "space-separated values"
		case "option":
			return "comma-separated options"
		case "version":
			return "comma-separated versions"
		case "component":
			return "comma-separated components"
		case "user":
			return "comma-separated email addresses"
		}
		return "array"
	case "doc":
		return "Markdown text"
	}
	return t
}

// parseIssueTemplate parses the YAML-like template content and converts values to Jira API format.
// Returns a map of field IDs to their API-formatted values.
func parseIssueTemplate(content string, ctx createIssueCtx, client *api.Client) (map[string]interface{}, error) {
	// Build a lookup map from fieldID to CreateField for type information
	fieldMeta := make(map[string]api.CreateField, len(ctx.fields))
	for _, f := range ctx.fields {
		fieldMeta[f.FieldID] = f
	}

	// Parse the YAML manually to preserve multi-line strings and skip comments.
	// We use a simple line-by-line parser for the key: value / key: | formats.
	parsed, err := parseSimpleYAML(content)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	result := make(map[string]interface{})

	for fieldID, rawValue := range parsed {
		value := strings.TrimSpace(rawValue)
		if value == "" {
			// Check if required
			if meta, ok := fieldMeta[fieldID]; ok && meta.Required {
				if fieldID != "project" && fieldID != "issuetype" {
					return nil, fmt.Errorf("required field %q (%s) is empty", fieldID, meta.Name)
				}
			}
			continue
		}

		meta, hasMeta := fieldMeta[fieldID]
		if !hasMeta {
			// Unknown field — include as plain string and hope for the best
			result[fieldID] = value
			continue
		}

		converted, err := convertFieldValue(value, meta, client)
		if err != nil {
			// Non-fatal: skip the field and continue
			continue
		}
		if converted != nil {
			result[fieldID] = converted
		}
	}

	return result, nil
}

// convertFieldValue converts a string value from the template to the Jira API format.
func convertFieldValue(value string, f api.CreateField, client *api.Client) (interface{}, error) {
	if isADFField(f) {
		return json.RawMessage(adf.FromMarkdown(value)), nil
	}

	switch f.Schema.Type {
	case "string":
		return value, nil

	case "number":
		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("field %s: invalid number %q", f.FieldID, value)
		}
		return n, nil

	case "date", "datetime":
		return value, nil

	case "priority":
		return map[string]string{"name": value}, nil

	case "option":
		return map[string]string{"value": value}, nil

	case "user":
		users, err := client.SearchUsers(value)
		if err != nil || len(users) == 0 {
			return nil, nil // skip silently
		}
		return map[string]string{"accountId": users[0].AccountID}, nil

	case "array":
		return convertArrayFieldValue(value, f, client)

	default:
		return value, nil
	}
}

// convertArrayFieldValue handles array-type fields.
func convertArrayFieldValue(value string, f api.CreateField, client *api.Client) (interface{}, error) {
	switch f.Schema.Items {
	case "string":
		// Space-separated (e.g. labels)
		parts := strings.Fields(value)
		if len(parts) == 0 {
			return nil, nil
		}
		return parts, nil

	case "option":
		parts := splitCSV(value)
		result := make([]map[string]string, 0, len(parts))
		for _, p := range parts {
			if p != "" {
				result = append(result, map[string]string{"value": p})
			}
		}
		if len(result) == 0 {
			return nil, nil
		}
		return result, nil

	case "version":
		parts := splitCSV(value)
		result := make([]map[string]string, 0, len(parts))
		for _, p := range parts {
			if p != "" {
				result = append(result, map[string]string{"name": p})
			}
		}
		if len(result) == 0 {
			return nil, nil
		}
		return result, nil

	case "component":
		parts := splitCSV(value)
		result := make([]map[string]string, 0, len(parts))
		for _, p := range parts {
			if p != "" {
				result = append(result, map[string]string{"name": p})
			}
		}
		if len(result) == 0 {
			return nil, nil
		}
		return result, nil

	case "user":
		parts := splitCSV(value)
		result := make([]map[string]string, 0, len(parts))
		for _, email := range parts {
			email = strings.TrimSpace(email)
			if email == "" {
				continue
			}
			users, err := client.SearchUsers(email)
			if err != nil || len(users) == 0 {
				continue
			}
			result = append(result, map[string]string{"accountId": users[0].AccountID})
		}
		if len(result) == 0 {
			return nil, nil
		}
		return result, nil

	default:
		// Fallback: space-separated strings
		parts := strings.Fields(value)
		if len(parts) == 0 {
			return nil, nil
		}
		return parts, nil
	}
}

// splitCSV splits a comma-separated string, trimming whitespace.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseSimpleYAML parses a simplified YAML file (key: value and key: | multiline).
// Comments (lines starting with #) are ignored.
// Returns a map of key → string value.
func parseSimpleYAML(content string) (map[string]string, error) {
	result := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentKey string
	var multilineLines []string
	inMultiline := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// If we're collecting multiline content
		if inMultiline {
			// A new key at the start of a line (not indented, not comment) ends multiline
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' && strings.Contains(line, ":") {
				// End current multiline
				result[currentKey] = strings.Join(multilineLines, "\n")
				multilineLines = nil
				inMultiline = false
				currentKey = ""
				// Fall through to process this line as a new key
			} else {
				// Collect multiline content (strip one level of indentation)
				stripped := line
				if len(line) >= 2 && (line[:2] == "  ") {
					stripped = line[2:]
				}
				multilineLines = append(multilineLines, stripped)
				continue
			}
		}

		// Skip comments and blank lines
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Parse key: value
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:colonIdx])
		rest := strings.TrimSpace(line[colonIdx+1:])

		if rest == "|" {
			// Block scalar — collect following indented lines
			currentKey = key
			inMultiline = true
			multilineLines = nil
		} else {
			// Strip surrounding quotes if present
			val := rest
			if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
				val = val[1 : len(val)-1]
			}
			result[key] = val
		}
	}

	// Flush any remaining multiline
	if inMultiline && currentKey != "" {
		result[currentKey] = strings.Join(multilineLines, "\n")
	}

	return result, nil
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

// loadBranchesCmd fetches the list of branches from Azure DevOps in preparation for creating a PR.
func loadBranchesCmd(client *api.AzdoClient, issueKey, issueSummary string) tea.Cmd {
	return func() tea.Msg {
		repoName := client.RepoName()
		if repoName == "" {
			repos, err := client.ListRepos()
			if err != nil {
				return errMsg{fmt.Errorf("listing repositories: %w", err)}
			}
			if len(repos) == 0 {
				return errMsg{fmt.Errorf("no repositories found in project")}
			}
			repoName = repos[0].Name
		}
		branches, err := client.ListBranches(repoName)
		if err != nil {
			return errMsg{fmt.Errorf("listing branches: %w", err)}
		}
		currentBranch, err := detectCurrentBranch()
		if err != nil {
			currentBranch = ""
		}
		return branchesLoadedMsg{
			branches:      branches,
			repoName:      repoName,
			issueKey:      issueKey,
			issueSummary:  issueSummary,
			currentBranch: currentBranch,
		}
	}
}

// createPRCmd creates a new pull request in Azure DevOps.
func createPRCmd(client *api.AzdoClient, repoName, title, description, sourceBranch, targetBranch string) tea.Cmd {
	return func() tea.Msg {
		sourceBranch = normalizeShortBranchName(sourceBranch)
		targetBranch = normalizeShortBranchName(targetBranch)
		if sourceBranch == "" {
			return errMsg{fmt.Errorf("source branch is required")}
		}
		if targetBranch == "" {
			return errMsg{fmt.Errorf("target branch is required")}
		}

		branches, err := client.ListBranches(repoName)
		if err != nil {
			return errMsg{fmt.Errorf("listing branches: %w", err)}
		}
		branchSet := make(map[string]struct{}, len(branches))
		for _, branch := range branches {
			branchSet[normalizeShortBranchName(branch)] = struct{}{}
		}
		if _, ok := branchSet[sourceBranch]; !ok {
			return errMsg{fmt.Errorf("source branch %q does not exist in repo %q", sourceBranch, repoName)}
		}
		if _, ok := branchSet[targetBranch]; !ok {
			return errMsg{fmt.Errorf("target branch %q does not exist in repo %q", targetBranch, repoName)}
		}

		pr, err := client.CreatePullRequest(repoName, title, description, sourceBranch, targetBranch)
		if err != nil {
			return errMsg{err}
		}
		return prCreatedMsg{pr: pr}
	}
}

func detectCurrentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("detached HEAD")
	}
	return branch, nil
}

func normalizeShortBranchName(branch string) string {
	branch = strings.TrimSpace(branch)
	branch = strings.TrimPrefix(branch, "refs/heads/")
	return branch
}
