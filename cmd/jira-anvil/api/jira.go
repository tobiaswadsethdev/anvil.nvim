package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a Jira Cloud REST API v3 client.
type Client struct {
	baseURL    string
	authHeader string
	http       *http.Client
	fields     []Field // cached field metadata
}

// NewClient creates a new Jira API client.
func NewClient(jiraURL, user, token string) *Client {
	creds := base64.StdEncoding.EncodeToString([]byte(user + ":" + token))
	return &Client{
		baseURL:    strings.TrimRight(jiraURL, "/"),
		authHeader: "Basic " + creds,
		http:       &http.Client{Timeout: 30 * time.Second},
	}
}

// --- Types ---

type Issue struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Self   string      `json:"self"`
	Fields IssueFields `json:"fields"`
}

type IssueFields struct {
	Summary     string          `json:"summary"`
	Description json.RawMessage `json:"description"` // ADF JSON
	Status      struct {
		Name string `json:"name"`
	} `json:"status"`
	Priority struct {
		Name string `json:"name"`
	} `json:"priority"`
	Assignee  *User           `json:"assignee"`
	Reporter  *User           `json:"reporter"`
	Created   time.Time       `json:"created"`
	Updated   time.Time       `json:"updated"`
	Comment   *CommentPage    `json:"comment"`
	Labels    []string        `json:"labels"`
	Custom    map[string]json.RawMessage `json:"-"` // populated by UnmarshalCustomFields
}

type User struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

type CommentPage struct {
	Comments []Comment `json:"comments"`
	Total    int       `json:"total"`
}

type Comment struct {
	ID      string          `json:"id"`
	Author  *User           `json:"author"`
	Body    json.RawMessage `json:"body"` // ADF JSON
	Created time.Time       `json:"created"`
	Updated time.Time       `json:"updated"`
}

type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   struct {
		Name string `json:"name"`
	} `json:"to"`
}

type Field struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Custom bool   `json:"custom"`
	Schema struct {
		Type   string `json:"type"`
		Custom string `json:"custom"`
	} `json:"schema"`
}

type SearchResult struct {
	Issues     []Issue `json:"issues"`
	Total      int     `json:"total"`
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
}

// --- Requests ---

func (c *Client) do(method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("jira API %s %s: %d %s", method, path, resp.StatusCode, string(respData))
	}
	return respData, nil
}

// --- API methods ---

// SearchIssues returns issues matching the given JQL query.
func (c *Client) SearchIssues(jql string, maxResults int) ([]Issue, int, error) {
	fields := "summary,status,priority,assignee,updated,labels"
	path := fmt.Sprintf("/rest/api/3/search?jql=%s&fields=%s&maxResults=%d",
		url.QueryEscape(jql), url.QueryEscape(fields), maxResults)

	data, err := c.do("GET", path, nil)
	if err != nil {
		return nil, 0, err
	}

	var result SearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, 0, err
	}
	return result.Issues, result.Total, nil
}

// GetIssue returns full issue details including description, comments, and custom fields.
func (c *Client) GetIssue(key string) (*Issue, error) {
	fields, err := c.GetFields()
	if err != nil {
		return nil, err
	}

	// Build field list: standard + all custom doc fields
	fieldList := "summary,description,status,priority,assignee,reporter,created,updated,comment,labels"
	for _, f := range fields {
		if f.Custom && f.Schema.Type == "doc" {
			fieldList += "," + f.ID
		}
	}

	data, err := c.do("GET", "/rest/api/3/issue/"+key+"?fields="+url.QueryEscape(fieldList), nil)
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, err
	}

	// Extract custom field values from raw JSON
	var raw struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(data, &raw); err == nil {
		issue.Fields.Custom = make(map[string]json.RawMessage)
		for _, f := range fields {
			if f.Custom && f.Schema.Type == "doc" {
				if v, ok := raw.Fields[f.ID]; ok {
					issue.Fields.Custom[f.Name] = v
				}
			}
		}
	}

	return &issue, nil
}

// GetTransitions returns available transitions for an issue.
func (c *Client) GetTransitions(key string) ([]Transition, error) {
	data, err := c.do("GET", "/rest/api/3/issue/"+key+"/transitions", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Transitions []Transition `json:"transitions"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Transitions, nil
}

// DoTransition executes a status transition on an issue.
func (c *Client) DoTransition(key, transitionID string) error {
	body := map[string]interface{}{
		"transition": map[string]string{"id": transitionID},
	}
	_, err := c.do("POST", "/rest/api/3/issue/"+key+"/transitions", body)
	return err
}

// AddComment posts a comment (ADF body) to an issue.
func (c *Client) AddComment(key string, adfBody json.RawMessage) error {
	body := map[string]interface{}{
		"body": json.RawMessage(adfBody),
	}
	_, err := c.do("POST", "/rest/api/3/issue/"+key+"/comment", body)
	return err
}

// UpdateIssue updates fields on an issue (summary, description, assignee, or custom ADF field).
func (c *Client) UpdateIssue(key string, fields map[string]interface{}) error {
	body := map[string]interface{}{"fields": fields}
	_, err := c.do("PUT", "/rest/api/3/issue/"+key, body)
	return err
}

// GetAssignableUsers returns users that can be assigned to an issue.
func (c *Client) GetAssignableUsers(issueKey string) ([]User, error) {
	path := "/rest/api/3/user/assignable/search?issueKey=" + url.QueryEscape(issueKey) + "&maxResults=50"
	data, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var users []User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// GetFields returns all field definitions (cached after first call).
func (c *Client) GetFields() ([]Field, error) {
	if c.fields != nil {
		return c.fields, nil
	}

	data, err := c.do("GET", "/rest/api/3/field", nil)
	if err != nil {
		return nil, err
	}

	var fields []Field
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	c.fields = fields
	return fields, nil
}

// AssignIssue sets the assignee of an issue. Pass empty accountID to unassign.
func (c *Client) AssignIssue(key, accountID string) error {
	var body map[string]interface{}
	if accountID == "" {
		body = map[string]interface{}{"accountId": nil}
	} else {
		body = map[string]interface{}{"accountId": accountID}
	}
	_, err := c.do("PUT", "/rest/api/3/issue/"+key+"/assignee", body)
	return err
}
