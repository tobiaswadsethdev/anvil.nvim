package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AzdoClient is an Azure DevOps REST API client using PAT authentication.
type AzdoClient struct {
	baseURL       string
	project       string
	repo          string
	authHeader    string
	http          *http.Client
	currentUserID string // cached after first GetCurrentUserID() call
}

// NewAzdoClient creates a new Azure DevOps API client.
// Auth uses Basic auth with an empty username and the PAT token as password.
func NewAzdoClient(url, project, repo, token string) *AzdoClient {
	creds := base64.StdEncoding.EncodeToString([]byte(":" + token))
	return &AzdoClient{
		baseURL:    strings.TrimRight(url, "/"),
		project:    project,
		repo:       repo,
		authHeader: "Basic " + creds,
		http:       &http.Client{Timeout: 30 * time.Second},
	}
}

// --- Types ---

// PullRequest represents an Azure DevOps pull request.
type PullRequest struct {
	PullRequestID int    `json:"pullRequestId"`
	Title         string `json:"title"`
	Status        string `json:"status"` // "active" | "completed" | "abandoned"
	CreatedBy     struct {
		DisplayName string `json:"displayName"`
	} `json:"createdBy"`
	CreationDate         time.Time `json:"creationDate"`
	SourceRefName        string    `json:"sourceRefName"` // "refs/heads/feature/CODE-123"
	TargetRefName        string    `json:"targetRefName"`
	LastMergeSourceCommit struct {
		CommitID string `json:"commitId"`
	} `json:"lastMergeSourceCommit"`
	LastMergeTargetCommit struct {
		CommitID string `json:"commitId"`
	} `json:"lastMergeTargetCommit"`
}

// Build represents an Azure DevOps pipeline run.
type Build struct {
	ID          int       `json:"id"`
	BuildNumber string    `json:"buildNumber"`
	Status      string    `json:"status"` // "completed" | "inProgress" | "notStarted"
	Result      string    `json:"result"` // "succeeded" | "failed" | "canceled" | "partiallySucceeded"
	StartTime   time.Time `json:"startTime"`
	Definition  struct {
		Name string `json:"name"`
	} `json:"definition"`
}

// ChangedFile is a file changed within a pull request.
type ChangedFile struct {
	Path             string
	ChangeType       string // "add" | "edit" | "delete" | "rename"
	ObjectID         string // target (source branch) blob SHA
	OriginalObjectID string // base (target branch) blob SHA
}

// DiffLine is a single line in a unified diff.
type DiffLine struct {
	Content string
	Type    string // "context" | "added" | "deleted"
}

// DiffHunk is a contiguous block of changes in a unified diff.
type DiffHunk struct {
	Header string
	Lines  []DiffLine
}

// FileDiff is the complete diff for a single file.
type FileDiff struct {
	Path       string
	ChangeType string
	Binary     bool
	Hunks      []DiffHunk // empty if Binary=true or no changes
}

// Reviewer represents an Azure DevOps PR reviewer and their vote status.
type Reviewer struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Vote        int    `json:"vote"` // -10=Rejected, -5=WaitingForAuthor, 0=NoVote, 5=ApprovedWithSuggestions, 10=Approved
}

// PRCommentThread is a conversation thread on a pull request.
type PRCommentThread struct {
	ID            int              `json:"id"`
	Status        string           `json:"status"`
	IsDeleted     bool             `json:"isDeleted"`
	Comments      []PRComment      `json:"comments"`
	ThreadContext *PRThreadContext `json:"threadContext"`
}

// PRComment is a single comment within a PRCommentThread.
type PRComment struct {
	ID              int    `json:"id"`
	Content         string `json:"content"`
	CommentType     string `json:"commentType"` // "text" | "system"
	ParentCommentID int    `json:"parentCommentId"`
	IsDeleted       bool   `json:"isDeleted"`
	Author          struct {
		DisplayName string `json:"displayName"`
	} `json:"author"`
	PublishedDate time.Time `json:"publishedDate"`
}

// PRThreadContext identifies the file and optional line range for a file/code comment.
type PRThreadContext struct {
	FilePath       string          `json:"filePath,omitempty"`
	RightFileStart *PRFilePosition `json:"rightFileStart,omitempty"`
	RightFileEnd   *PRFilePosition `json:"rightFileEnd,omitempty"`
}

// PRFilePosition is a line/offset within a file for a code comment.
type PRFilePosition struct {
	Line   int `json:"line"`
	Offset int `json:"offset"`
}

// PRWebURL returns the browser-accessible URL for a pull request.
func (c *AzdoClient) PRWebURL(prID int) string {
	return fmt.Sprintf("%s/%s/_git/%s/pullrequest/%d", c.baseURL, c.project, c.repo, prID)
}

// --- Internal helpers ---

func (c *AzdoClient) repoURL() string {
	return fmt.Sprintf("%s/%s/_apis/git/repositories/%s", c.baseURL, c.project, c.repo)
}

func (c *AzdoClient) buildURL() string {
	return fmt.Sprintf("%s/%s/_apis/build", c.baseURL, c.project)
}

func (c *AzdoClient) get(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		msg := string(body)
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		return nil, fmt.Errorf("azure devops %s: %s", resp.Status, msg)
	}
	return body, nil
}

func (c *AzdoClient) getRaw(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("azure devops blob HTTP %s", resp.Status)
	}
	return body, nil
}

func (c *AzdoClient) post(url string, body []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		msg := string(respBody)
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		return nil, fmt.Errorf("azure devops %s: %s", resp.Status, msg)
	}
	return respBody, nil
}

func (c *AzdoClient) put(url string, body []byte) ([]byte, error) {
	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		msg := string(respBody)
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		return nil, fmt.Errorf("azure devops %s: %s", resp.Status, msg)
	}
	return respBody, nil
}

// --- Public API ---

// GetPRByIssueKey finds the most recent PR whose source branch contains the given Jira issue key.
// Returns nil, nil if no matching PR is found.
func (c *AzdoClient) GetPRByIssueKey(issueKey string) (*PullRequest, error) {
	url := fmt.Sprintf("%s/pullrequests?searchCriteria.status=all&$top=50&api-version=7.1", c.repoURL())
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []PullRequest `json:"value"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing pull requests: %w", err)
	}

	for i := range result.Value {
		if strings.Contains(result.Value[i].SourceRefName, issueKey) {
			pr := result.Value[i]
			return &pr, nil
		}
	}
	return nil, nil
}

// GetChangedFiles returns the list of files changed between the PR's source and target branches.
func (c *AzdoClient) GetChangedFiles(pr *PullRequest) ([]ChangedFile, error) {
	base := strings.TrimPrefix(pr.TargetRefName, "refs/heads/")
	target := strings.TrimPrefix(pr.SourceRefName, "refs/heads/")

	url := fmt.Sprintf(
		"%s/diffs/commits?baseVersion=%s&targetVersion=%s&baseVersionType=Branch&targetVersionType=Branch&$top=200&api-version=7.1",
		c.repoURL(), base, target,
	)
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}

	var result struct {
		Changes []struct {
			Item struct {
				Path             string `json:"path"`
				ObjectID         string `json:"objectId"`
				OriginalObjectID string `json:"originalObjectId"`
			} `json:"item"`
			ChangeType string `json:"changeType"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing diff: %w", err)
	}

	files := make([]ChangedFile, 0, len(result.Changes))
	for _, ch := range result.Changes {
		files = append(files, ChangedFile{
			Path:             ch.Item.Path,
			ChangeType:       ch.ChangeType,
			ObjectID:         ch.Item.ObjectID,
			OriginalObjectID: ch.Item.OriginalObjectID,
		})
	}
	return files, nil
}

// GetBlob fetches the raw content of a git blob by its SHA.
// Returns empty string if objectID is empty.
func (c *AzdoClient) GetBlob(objectID string) (string, error) {
	if objectID == "" {
		return "", nil
	}
	url := fmt.Sprintf("%s/blobs/%s?api-version=7.1", c.repoURL(), objectID)
	body, err := c.getRaw(url)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// GetLatestBuild returns the most recent pipeline run for the PR's source branch.
func (c *AzdoClient) GetLatestBuild(sourceRefName string) (*Build, error) {
	branch := sourceRefName
	if !strings.HasPrefix(branch, "refs/heads/") {
		branch = "refs/heads/" + branch
	}
	url := fmt.Sprintf("%s/builds?branchName=%s&$top=1&api-version=7.1", c.buildURL(), branch)
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []Build `json:"value"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing builds: %w", err)
	}
	if len(result.Value) == 0 {
		return nil, nil
	}
	b := result.Value[0]
	return &b, nil
}

// GetReviewers returns the list of reviewers and their votes for a pull request.
func (c *AzdoClient) GetReviewers(pr *PullRequest) ([]Reviewer, error) {
	url := fmt.Sprintf("%s/pullRequests/%d/reviewers?api-version=7.1", c.repoURL(), pr.PullRequestID)
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []Reviewer `json:"value"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing reviewers: %w", err)
	}
	return result.Value, nil
}

// GetCurrentUserID returns the authenticated user's Azure DevOps identity ID.
// The result is cached on the client after the first call.
func (c *AzdoClient) GetCurrentUserID() (string, error) {
	if c.currentUserID != "" {
		return c.currentUserID, nil
	}
	url := fmt.Sprintf("%s/_apis/connectionData", c.baseURL)
	body, err := c.get(url)
	if err != nil {
		return "", err
	}

	var result struct {
		AuthenticatedUser struct {
			ID string `json:"id"`
		} `json:"authenticatedUser"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing connectionData: %w", err)
	}
	c.currentUserID = result.AuthenticatedUser.ID
	return c.currentUserID, nil
}

// SubmitVote casts a vote on a pull request as the given reviewer.
// vote: -10=Rejected, -5=WaitingForAuthor, 0=NoVote, 5=ApprovedWithSuggestions, 10=Approved
func (c *AzdoClient) SubmitVote(pr *PullRequest, vote int, reviewerID string) error {
	url := fmt.Sprintf("%s/pullRequests/%d/reviewers/%s?api-version=7.1", c.repoURL(), pr.PullRequestID, reviewerID)
	payload, err := json.Marshal(map[string]int{"vote": vote})
	if err != nil {
		return err
	}
	_, err = c.put(url, payload)
	return err
}

// GetPRThreads returns all non-system, non-deleted comment threads for a pull request.
func (c *AzdoClient) GetPRThreads(pr *PullRequest) ([]PRCommentThread, error) {
	url := fmt.Sprintf("%s/pullRequests/%d/threads?api-version=7.1", c.repoURL(), pr.PullRequestID)
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []PRCommentThread `json:"value"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing PR threads: %w", err)
	}

	// Filter out deleted threads and system-generated threads.
	threads := result.Value[:0]
	for _, t := range result.Value {
		if t.IsDeleted {
			continue
		}
		if len(t.Comments) > 0 && t.Comments[0].CommentType == "system" {
			continue
		}
		threads = append(threads, t)
	}
	return threads, nil
}

// AddPRThread creates a new comment thread on a pull request.
// ctx is nil for a general (PR-level) comment, or contains file/line info for file/code comments.
func (c *AzdoClient) AddPRThread(pr *PullRequest, content string, ctx *PRThreadContext) error {
	type comment struct {
		Content         string `json:"content"`
		CommentType     string `json:"commentType"`
		ParentCommentID int    `json:"parentCommentId"`
	}
	type thread struct {
		Comments      []comment        `json:"comments"`
		Status        string           `json:"status"`
		ThreadContext *PRThreadContext `json:"threadContext,omitempty"`
	}

	payload, err := json.Marshal(thread{
		Comments:      []comment{{Content: content, CommentType: "text", ParentCommentID: 0}},
		Status:        "active",
		ThreadContext: ctx,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/pullRequests/%d/threads?api-version=7.1", c.repoURL(), pr.PullRequestID)
	_, err = c.post(url, payload)
	return err
}

// ReplyToPRThread adds a reply comment to an existing thread.
// parentCommentID should be the ID of the root comment in the thread.
func (c *AzdoClient) ReplyToPRThread(pr *PullRequest, threadID, parentCommentID int, content string) error {
	type comment struct {
		Content         string `json:"content"`
		CommentType     string `json:"commentType"`
		ParentCommentID int    `json:"parentCommentId"`
	}

	payload, err := json.Marshal(comment{
		Content:         content,
		CommentType:     "text",
		ParentCommentID: parentCommentID,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/pullRequests/%d/threads/%d/comments?api-version=7.1", c.repoURL(), pr.PullRequestID, threadID)
	_, err = c.post(url, payload)
	return err
}
