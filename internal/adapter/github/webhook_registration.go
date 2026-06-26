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

// DeleteWebhook deletes a GitHub webhook for the given repository.
// It obtains an installation access token via the provided AppAuth and
// installation ID, then calls DELETE /repos/{owner}/{repo}/hooks/{webhookID}.
// Treats 404 as success (webhook already deleted).
func DeleteWebhook(ctx context.Context, appAuth *AppAuth, installationID int, owner, repo string, webhookID int) error {
	token, err := appAuth.GetToken(ctx, strconv.Itoa(installationID))
	if err != nil {
		return fmt.Errorf("delete webhook: get token: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/hooks/%d", appAuth.baseURL, owner, repo, webhookID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("delete webhook: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := appAuth.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete webhook: execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 404 means already deleted — treat as success.
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete webhook: HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// UpdateWebhook updates a GitHub webhook's configuration (URL and secret).
// It obtains an installation access token via the provided AppAuth and
// installation ID, then calls PATCH /repos/{owner}/{repo}/hooks/{webhookID}.
func UpdateWebhook(ctx context.Context, appAuth *AppAuth, installationID int, owner, repo string, webhookID int, newURL, newSecret string) error {
	token, err := appAuth.GetToken(ctx, strconv.Itoa(installationID))
	if err != nil {
		return fmt.Errorf("update webhook: get token: %w", err)
	}

	payload := map[string]interface{}{
		"config": map[string]string{
			"url":          newURL,
			"secret":       newSecret,
			"content_type": "json",
		},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("update webhook: marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/hooks/%d", appAuth.baseURL, owner, repo, webhookID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("update webhook: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := appAuth.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("update webhook: execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update webhook: HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// VerifyWebhook checks that a GitHub webhook still exists.
// It calls GET /repos/{owner}/{repo}/hooks/{webhookID} using the
// provided AppAuth token. Returns nil on 200 (webhook exists),
// and an error on any other status (404 means the webhook is gone).
func VerifyWebhook(ctx context.Context, appAuth *AppAuth, installationID int, owner, repo string, webhookID int) error {
	token, err := appAuth.GetToken(ctx, strconv.Itoa(installationID))
	if err != nil {
		return fmt.Errorf("verify webhook: get token: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/hooks/%d", appAuth.baseURL, owner, repo, webhookID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("verify webhook: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := appAuth.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("verify webhook: execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("verify webhook: HTTP %d: %s", resp.StatusCode, string(body))
}
