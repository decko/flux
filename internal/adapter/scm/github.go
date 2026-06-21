package scm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/decko/flux/internal/adapter/github"
	"github.com/decko/flux/internal/model"
)

// GitHub API response types for pull requests and reviews.
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

// ListPullRequests returns all pull requests for the repository.
// It paginates through the GitHub API and extracts ticket references
// from PR body text.
func (a *GitHubSCMAdapter) ListPullRequests(ctx context.Context, projectID string) ([]model.PullRequest, error) {
	var allPRs []model.PullRequest
	url := a.baseURL + "/repos/" + a.owner + "/" + a.repo + "/pulls?state=all"

	for url != "" {
		resp, err := a.ghClient.DoRequest(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("list pull requests: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("list pull requests: unexpected status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
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

		url = github.GetNextPageURL(resp)
	}

	return allPRs, nil
}

// GetPullRequest returns a single pull request from GitHub by its number.
// Returns nil, error if the pull request is not found (404).
func (a *GitHubSCMAdapter) GetPullRequest(ctx context.Context, projectID, externalID string) (*model.PullRequest, error) {
	if err := validateExternalID(externalID); err != nil {
		return nil, fmt.Errorf("get pull request: %w", err)
	}

	url := a.baseURL + "/repos/" + a.owner + "/" + a.repo + "/pulls/" + externalID

	resp, err := a.ghClient.DoRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("get pull request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

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
// It paginates through the GitHub API like ListPullRequests.
func (a *GitHubSCMAdapter) ListReviews(ctx context.Context, projectID, externalID string) ([]model.Review, error) {
	if err := validateExternalID(externalID); err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}

	var allReviews []model.Review
	url := a.baseURL + "/repos/" + a.owner + "/" + a.repo + "/pulls/" + externalID + "/reviews"

	for url != "" {
		resp, err := a.ghClient.DoRequest(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("list reviews: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("list reviews: unexpected status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		if err != nil {
			return nil, fmt.Errorf("list reviews: %w", err)
		}

		var reviews []ghReviewResponse
		if err := json.Unmarshal(body, &reviews); err != nil {
			return nil, fmt.Errorf("list reviews: %w", err)
		}

		for _, r := range reviews {
			var status model.ReviewStatus
			switch r.State {
			case "APPROVED":
				status = model.ReviewStatusApproved
			case "CHANGES_REQUESTED":
				status = model.ReviewStatusChangesRequested
			case "COMMENTED":
				status = model.ReviewStatusCommented
			default:
				// Skip unknown review states (PENDING, DISMISSED, etc.).
				continue
			}
			createdAt, _ := time.Parse(time.RFC3339, r.SubmittedAt)

			allReviews = append(allReviews, model.Review{
				Author:    r.User.Login,
				Status:    status,
				Comment:   r.Body,
				CreatedAt: createdAt,
			})
		}

		url = github.GetNextPageURL(resp)
	}

	return allReviews, nil
}

// Health checks connectivity to the GitHub API by querying the repository.
func (a *GitHubSCMAdapter) Health(ctx context.Context) error {
	url := a.baseURL + "/repos/" + a.owner + "/" + a.repo

	resp, err := a.ghClient.DoRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check: unexpected status %d", resp.StatusCode)
	}

	return nil
}

// validateExternalID checks that externalID is a valid numeric string.
// This prevents path traversal and ensures the ID is safe for URL construction.
func validateExternalID(externalID string) error {
	_, err := strconv.Atoi(externalID)
	if err != nil {
		return fmt.Errorf("invalid external ID %q: must be numeric", externalID)
	}
	return nil
}

// parseTime parses an RFC3339 timestamp and logs a warning if parsing fails.
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		slog.Warn("failed to parse timestamp", "value", s, "error", err)
		return time.Time{}
	}
	return t
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

	result.CreatedAt = parseTime(pr.CreatedAt)
	result.UpdatedAt = parseTime(pr.UpdatedAt)

	matches := ticketRefRE.FindAllStringSubmatch(pr.Body, -1)
	for _, m := range matches {
		result.TicketIDs = append(result.TicketIDs, m[1])
	}

	return result
}
