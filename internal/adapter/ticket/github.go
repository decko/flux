// Package ticket provides adapter implementations for external issue
// tracking systems.
package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/decko/flux/internal/model"
)

// ErrRateLimited is returned when the GitHub API rate limit is exceeded.
var ErrRateLimited = errors.New("rate limit exceeded")

// ghLabel represents a label in the GitHub API response.
type ghLabel struct {
	Name string `json:"name"`
}

// ghIssue represents an issue in the GitHub API response. Only fields
// relevant to the adapter are included.
type ghIssue struct {
	Number int       `json:"number"`
	Title  string    `json:"title"`
	Body   string    `json:"body"`
	State  string    `json:"state"`
	Labels []ghLabel `json:"labels"`
}

// createIssueRequest is the JSON body for creating a GitHub issue.
type createIssueRequest struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels,omitempty"`
}

// updateIssueRequest is the JSON body for updating a GitHub issue.
type updateIssueRequest struct {
	State  string   `json:"state"`
	Labels []string `json:"labels,omitempty"`
}

// GitHubAdapter implements TicketAdapter for GitHub Issues.
type GitHubAdapter struct {
	owner      string
	repo       string
	token      string
	baseURL    string
	httpClient *http.Client
}

// GitHubAdapterOption configures a GitHubAdapter.
type GitHubAdapterOption func(*GitHubAdapter)

// WithBaseURL sets the base URL for the GitHub API. Used for testing with
// httptest servers.
func WithBaseURL(baseURL string) GitHubAdapterOption {
	return func(a *GitHubAdapter) {
		a.baseURL = baseURL
	}
}

// NewGitHubAdapter creates a new GitHubAdapter. If httpClient is nil,
// http.DefaultClient is used. The default base URL is https://api.github.com.
func NewGitHubAdapter(owner, repo, token string, httpClient *http.Client, opts ...GitHubAdapterOption) *GitHubAdapter {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	a := &GitHubAdapter{
		owner:      owner,
		repo:       repo,
		token:      token,
		baseURL:    "https://api.github.com",
		httpClient: httpClient,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Name returns "github".
func (a *GitHubAdapter) Name() string {
	return "github"
}

// ListTickets returns all GitHub issues for the repository.
func (a *GitHubAdapter) ListTickets(ctx context.Context, projectID string) ([]model.Ticket, error) {
	var all []model.Ticket
	url := a.issuesURL()
	for url != "" {
		page, next, err := a.listPage(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("list tickets: %w", err)
		}
		all = append(all, page...)
		url = next
	}
	if all == nil {
		return []model.Ticket{}, nil
	}
	return all, nil
}

// GetTicket retrieves a single GitHub issue by its number.
func (a *GitHubAdapter) GetTicket(ctx context.Context, projectID, externalID string) (*model.Ticket, error) {
	url := fmt.Sprintf("%s/%s", a.issuesURL(), externalID)
	resp, err := a.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("get ticket: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("get ticket: not found")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get ticket: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var issue ghIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("get ticket: decode response: %w", err)
	}
	ticket := a.toTicket(issue)
	return &ticket, nil
}

// CreateTicket creates a new GitHub issue and returns the created ticket with
// server-assigned fields populated.
func (a *GitHubAdapter) CreateTicket(ctx context.Context, ticket *model.Ticket) (*model.Ticket, error) {
	body := createIssueRequest{
		Title:  ticket.Title,
		Body:   ticket.Description,
		Labels: ticket.Labels,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("create ticket: encode body: %w", err)
	}

	resp, err := a.doRequest(ctx, http.MethodPost, a.issuesURL(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create ticket: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var issue ghIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("create ticket: decode response: %w", err)
	}
	created := a.toTicket(issue)
	return &created, nil
}

// UpdateTicket modifies an existing GitHub issue. Status is mapped from
// TicketStatus to GitHub state: open, in_progress → "open", closed → "closed".
func (a *GitHubAdapter) UpdateTicket(ctx context.Context, ticket *model.Ticket) error {
	url := fmt.Sprintf("%s/%s", a.issuesURL(), ticket.ExternalID)

	state := "open"
	if ticket.Status == model.TicketStatusClosed {
		state = "closed"
	}

	body := updateIssueRequest{
		State:  state,
		Labels: ticket.Labels,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("update ticket: encode body: %w", err)
	}

	resp, err := a.doRequest(ctx, http.MethodPatch, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("update ticket: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update ticket: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// SyncRelationships is not yet implemented. Relationship parsing will be
// handled in a future issue.
func (a *GitHubAdapter) SyncRelationships(ctx context.Context, projectID string) error {
	return nil
}

// Health checks whether the repository is accessible on GitHub.
func (a *GitHubAdapter) Health(ctx context.Context) error {
	resp, err := a.doRequest(ctx, http.MethodGet, a.repoURL(), nil)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// doRequest creates and executes an HTTP request with GitHub authentication
// headers. It checks the X-RateLimit-Remaining header and returns
// ErrRateLimited if the limit is exhausted.
func (a *GitHubAdapter) doRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}

	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		_ = resp.Body.Close()
		return nil, ErrRateLimited
	}

	return resp, nil
}

// issuesURL returns the base URL for the issues API endpoint.
func (a *GitHubAdapter) issuesURL() string {
	return fmt.Sprintf("%s/repos/%s/%s/issues", a.baseURL, a.owner, a.repo)
}

// repoURL returns the URL for the repository API endpoint.
func (a *GitHubAdapter) repoURL() string {
	return fmt.Sprintf("%s/repos/%s/%s", a.baseURL, a.owner, a.repo)
}

// listPage fetches one page of issues from the given URL and returns the
// parsed tickets along with the next page URL (from the Link header), if any.
func (a *GitHubAdapter) listPage(ctx context.Context, url string) ([]model.Ticket, string, error) {
	resp, err := a.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var issues []ghIssue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, "", fmt.Errorf("decode response: %w", err)
	}

	tickets := make([]model.Ticket, len(issues))
	for i, issue := range issues {
		tickets[i] = a.toTicket(issue)
	}

	nextURL := getNextPageURL(resp.Header.Get("Link"))
	return tickets, nextURL, nil
}

// toTicket converts a ghIssue API response to a model.Ticket.
func (a *GitHubAdapter) toTicket(issue ghIssue) model.Ticket {
	status := model.TicketStatusClosed
	if issue.State == "open" {
		status = model.TicketStatusOpen
	}
	labels := make([]string, len(issue.Labels))
	for i, l := range issue.Labels {
		labels[i] = l.Name
	}
	return model.Ticket{
		ExternalID:  strconv.Itoa(issue.Number),
		Title:       issue.Title,
		Description: issue.Body,
		Status:      status,
		Labels:      labels,
		Source:      model.TicketSourceGitHub,
	}
}

// getNextPageURL extracts the next page URL from a Link header value.
// It returns the URL marked with rel="next", or empty string if absent.
func getNextPageURL(link string) string {
	if link == "" {
		return ""
	}
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, `rel="next"`) {
			start := strings.Index(part, "<")
			end := strings.Index(part, ">")
			if start != -1 && end != -1 && end > start {
				return part[start+1 : end]
			}
		}
	}
	return ""
}

// Compile-time check: GitHubAdapter satisfies TicketAdapter.
var _ TicketAdapter = (*GitHubAdapter)(nil)
