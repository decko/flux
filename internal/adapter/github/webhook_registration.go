package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// WebhookConfig is the configuration payload for creating a GitHub webhook.
type WebhookConfig struct {
	URL         string   `json:"url"`
	ContentType string   `json:"content_type"`
	Secret      string   `json:"secret"`
	InsecureSSL string   `json:"insecure_ssl,omitempty"`
	Events      []string `json:"events"`
}

// webhookRequest is the full request body for the GitHub webhook API.
type webhookRequest struct {
	Name   string        `json:"name"`
	Active bool          `json:"active"`
	Events []string      `json:"events"`
	Config WebhookConfig `json:"config"`
}

// webhookResponse is the response from the GitHub webhook API.
type webhookResponse struct {
	ID int `json:"id"`
}

// RegisterWebhook creates a GitHub webhook for a repository using the
// given installation access token. It POSTs to /repos/{owner}/{repo}/hooks
// with the provided configuration and returns the webhook ID.
//
// The AppAuth is used to get an installation token for the given
// installationID. The webhook is configured with content_type=json,
// the provided events, and active=true.
func RegisterWebhook(ctx context.Context, appAuth *AppAuth, installationID, owner, repo, webhookURL, secret string) (int, error) {
	token, err := appAuth.GetToken(ctx, installationID)
	if err != nil {
		return 0, fmt.Errorf("webhook registration: get installation token: %w", err)
	}

	client := NewClient(token, nil)

	body := webhookRequest{
		Name:   "web",
		Active: true,
		Events: []string{"issues", "pull_request", "push"},
		Config: WebhookConfig{
			URL:         webhookURL,
			ContentType: "json",
			Secret:      secret,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("webhook registration: marshal request: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks", owner, repo)
	resp, err := client.DoRequest(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("webhook registration: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("webhook registration: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result webhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("webhook registration: decode response: %w", err)
	}

	return result.ID, nil
}
