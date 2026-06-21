// Package github provides shared HTTP client utilities for GitHub API adapters
// used by both SCM (pull requests) and ticket (issue) adapters.
package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ErrRateLimited is returned when the GitHub API rate limit has been exceeded.
var ErrRateLimited = errors.New("github API rate limit exceeded")

// Client is a minimal HTTP client wrapper for GitHub REST API v3 communication.
// It handles authentication headers, accept headers, and rate-limit checking.
type Client struct {
	token      string
	httpClient *http.Client
}

// NewClient creates a new Client. If httpClient is nil, http.DefaultClient is used.
func NewClient(token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		token:      token,
		httpClient: httpClient,
	}
}

// DoRequest creates and executes an HTTP request with GitHub authentication
// headers. It checks the X-RateLimit-Remaining header and returns
// ErrRateLimited if the remaining count is zero. The caller must close
// resp.Body on success.
func (c *Client) DoRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}

	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		_ = resp.Body.Close()
		return nil, ErrRateLimited
	}

	return resp, nil
}

// GetNextPageURL extracts the URL with rel="next" from the Link header.
// Returns empty string if no next page is available.
func GetNextPageURL(resp *http.Response) string {
	link := resp.Header.Get("Link")
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
