package scm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/decko/flux/internal/model"
)

// ErrRateLimited is returned when the GitHub API rate limit has been exceeded.
var ErrRateLimited = fmt.Errorf("github API rate limit exceeded")

// GitHub API response types.
type (
	ghPRResponse struct {
		Number    int     `json:"number"`
		Title     string  `json:"title"`
		HTMLURL   string  `json:"html_url"`
		State     string  `json:"state"`
		MergedAt  *string `json:"merged_at"`
		Body      string  `json:"body"`
		CreatedAt string  `json:"created_at"`
		UpdatedAt string  `json:"updated_at"`
	}

	ghReviewResponse struct {
		User        ghUserResponse `json:"user"`
		State       string         `json:"state"`
		Body        string         `json:"body"`
		SubmittedAt string         `json:"submitted_at"`
	}

	ghUserResponse struct {
		Login string `json:"login"`
	}
)

// ticketRefRE matches GitHub issue reference patterns in PR bodies:
// closes #N, fixes #N, refs #N (case-insensitive).
var ticketRefRE = regexp.MustCompile(`(?i)(?:closes|fixes|refs)\s+#(\d+)`)

// Compile-time check: GitHubSCMAdapter satisfies SCMAdapter.
var _ SCMAdapter = (*GitHubSCMAdapter)(nil)

// WithBaseURL sets a custom base URL on the adapter.
// Useful for testing or GitHub Enterprise.
func WithBaseURL(a *GitHubSCMAdapter, baseURL string) {
	a.baseURL = strings.TrimRight(baseURL, "/")
}

// ListPullRequests returns all pull requests for the repository.
// It paginates through the GitHub API and extracts ticket references
// from PR body text.
func (a *GitHubSCMAdapter) ListPullRequests(ctx context.Context, projectID string) ([]model.PullRequest, error) {
	var allPRs []model.PullRequest
	url := a.baseURL + "/repos/" + a.owner + "/" + a.repo + "/pulls?state=all"

	for url != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("list pull requests: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+a.token)
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		resp, err := a.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("list pull requests: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if err := checkRateLimit(resp); err != nil {
			return nil, fmt.Errorf("list pull requests: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("list pull requests: unexpected status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("list pull requests: %w", err)
		}

		var prs []ghPRResponse
		if err := json.Unmarshal(body, &prs); err != nil {
			return nil, fmt.Errorf("list pull requests: %w", err)
		}

		for _, pr := range prs {
			allPRs = append(allPRs, convertPR(pr, projectID))
		}

		url = getNextPageURL(resp)
	}

	return allPRs, nil
}

// GetPullRequest returns a single pull request from GitHub by its number.
// Returns nil, error if the pull request is not found (404).
func (a *GitHubSCMAdapter) GetPullRequest(ctx context.Context, projectID, externalID string) (*model.PullRequest, error) {
	url := a.baseURL + "/repos/" + a.owner + "/" + a.repo + "/pulls/" + externalID

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("get pull request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get pull request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("get pull request: not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get pull request: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("get pull request: %w", err)
	}

	var pr ghPRResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("get pull request: %w", err)
	}

	result := convertPR(pr, projectID)
	return &result, nil
}

// ListReviews returns all reviews for the specified pull request.
func (a *GitHubSCMAdapter) ListReviews(ctx context.Context, projectID, externalID string) ([]model.Review, error) {
	url := a.baseURL + "/repos/" + a.owner + "/" + a.repo + "/pulls/" + externalID + "/reviews"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list reviews: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}

	var reviews []ghReviewResponse
	if err := json.Unmarshal(body, &reviews); err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}

	result := make([]model.Review, 0, len(reviews))
	for _, r := range reviews {
		var status model.ReviewStatus
		switch r.State {
		case "APPROVED":
			status = model.ReviewStatusApproved
		case "CHANGES_REQUESTED":
			status = model.ReviewStatusChangesRequested
		case "COMMENTED":
			status = model.ReviewStatusCommented
		}
		createdAt, _ := time.Parse(time.RFC3339, r.SubmittedAt)

		result = append(result, model.Review{
			Author:    r.User.Login,
			Status:    status,
			Comment:   r.Body,
			CreatedAt: createdAt,
		})
	}

	return result, nil
}

// Health checks connectivity to the GitHub API by querying the repository.
func (a *GitHubSCMAdapter) Health(ctx context.Context) error {
	url := a.baseURL + "/repos/" + a.owner + "/" + a.repo

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check: unexpected status %d", resp.StatusCode)
	}

	return nil
}

// checkRateLimit checks the X-RateLimit-Remaining header and returns
// ErrRateLimited if the remaining count is zero.
func checkRateLimit(resp *http.Response) error {
	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return ErrRateLimited
	}
	return nil
}

// getNextPageURL extracts the URL with rel="next" from the Link header.
// Returns empty string if no next page is available.
func getNextPageURL(resp *http.Response) string {
	link := resp.Header.Get("Link")
	if link == "" {
		return ""
	}
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, `rel="next"`) {
			start := strings.Index(part, "<")
			end := strings.Index(part, ">")
			if start != -1 && end != -1 {
				return part[start+1 : end]
			}
		}
	}
	return ""
}

// convertPR maps a GitHub API pull request response to a model.PullRequest.
func convertPR(pr ghPRResponse, projectID string) model.PullRequest {
	result := model.PullRequest{
		ExternalID: strconv.Itoa(pr.Number),
		ProjectID:  projectID,
		Source:     model.PRSourceGitHub,
		Title:      pr.Title,
		URL:        pr.HTMLURL,
	}

	switch {
	case pr.State == "open":
		result.Status = model.PRStatusOpen
	case pr.State == "closed" && pr.MergedAt != nil:
		result.Status = model.PRStatusMerged
	case pr.State == "closed":
		result.Status = model.PRStatusClosed
	}

	result.CreatedAt, _ = time.Parse(time.RFC3339, pr.CreatedAt)
	result.UpdatedAt, _ = time.Parse(time.RFC3339, pr.UpdatedAt)

	matches := ticketRefRE.FindAllStringSubmatch(pr.Body, -1)
	for _, m := range matches {
		result.TicketIDs = append(result.TicketIDs, m[1])
	}

	return result
}
