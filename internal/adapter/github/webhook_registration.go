package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// RegisterWebhook creates a GitHub webhook for a repository using the GitHub App
// installation token. Returns the webhook ID on success.
func RegisterWebhook(ctx context.Context, appAuth *AppAuth, installationID int, owner, repo, webhookURL, secret string) (int, error) {
	token, err := appAuth.GetToken(ctx, strconv.Itoa(installationID))
	if err != nil {
		return 0, fmt.Errorf("webhook register: get token: %w", err)
	}

	payload := map[string]interface{}{
		"name":   "web",
		"active": true,
		"events": []string{"issues", "pull_request", "push"},
		"config": map[string]interface{}{
			"url":          webhookURL,
			"content_type": "json",
			"secret":       secret,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("webhook register: marshal payload: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("webhook register: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("webhook register: execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("webhook register: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("webhook register: decode response: %w", err)
	}

	return result.ID, nil
}

// DeleteWebhook deletes a GitHub webhook by ID. Treats 404 as success.
func DeleteWebhook(ctx context.Context, appAuth *AppAuth, installationID int, owner, repo string, webhookID int) error {
	token, err := appAuth.GetToken(ctx, strconv.Itoa(installationID))
	if err != nil {
		return fmt.Errorf("webhook delete: get token: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks/%d", owner, repo, webhookID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("webhook delete: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook delete: execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil // already deleted
	}
	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook delete: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// VerifyWebhook checks if a webhook still exists. Returns nil on 200.
func VerifyWebhook(ctx context.Context, appAuth *AppAuth, installationID int, owner, repo string, webhookID int) error {
	token, err := appAuth.GetToken(ctx, strconv.Itoa(installationID))
	if err != nil {
		return fmt.Errorf("webhook verify: get token: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks/%d", owner, repo, webhookID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("webhook verify: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook verify: execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("webhook not found")
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook verify: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
