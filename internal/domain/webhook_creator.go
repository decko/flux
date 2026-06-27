package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/decko/flux/internal/adapter/github"
	"github.com/decko/flux/internal/model"
)

// webhookProjectRepo is the subset of ProjectRepository needed by
// WebhookCreator.
type webhookProjectRepo interface {
	Get(ctx context.Context, id string) (model.Project, error)
	Update(ctx context.Context, project model.Project) error
}

// webhookSecretRepo is the subset of WebhookSecretRepository needed by
// WebhookCreator.
type webhookSecretRepo interface {
	Set(ctx context.Context, repoURL, secret string) error
}

// WebhookCreator handles automatic GitHub webhook registration for projects.
// It generates a random secret, registers the webhook with GitHub, and stores
// the webhook ID on the project.
type WebhookCreator struct {
	appAuth      *github.AppAuth
	projectRepo  webhookProjectRepo
	secretRepo   webhookSecretRepo
	webhookURLFn func() string
}

// GitHubWebhookUpdater is a concrete implementation of WebhookUpdater that
// uses a *github.AppAuth to call the GitHub API.
type GitHubWebhookUpdater struct {
	AppAuth *github.AppAuth
}

// UpdateWebhook updates the config of an existing GitHub webhook.
func (u *GitHubWebhookUpdater) UpdateWebhook(ctx context.Context, installationID int, owner, repo string, webhookID int, webhookURL, secret string) error {
	if u.AppAuth == nil {
		return fmt.Errorf("github app not configured")
	}
	return github.UpdateWebhook(ctx, u.AppAuth, installationID, owner, repo, webhookID, webhookURL, secret)
}

// NewWebhookCreator creates a new WebhookCreator. The webhookURLFn is called
// to get the public URL for webhook delivery (defaults to reading
// FLUX_WEBHOOK_URL from the environment).
func NewWebhookCreator(
	appAuth *github.AppAuth,
	projectRepo webhookProjectRepo,
	secretRepo webhookSecretRepo,
	webhookURLFn func() string,
) *WebhookCreator {
	if webhookURLFn == nil {
		webhookURLFn = func() string {
			return os.Getenv("FLUX_WEBHOOK_URL")
		}
	}
	return &WebhookCreator{
		appAuth:      appAuth,
		projectRepo:  projectRepo,
		secretRepo:   secretRepo,
		webhookURLFn: webhookURLFn,
	}
}

// generateSecret generates a cryptographically random 32-byte hex string.
func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate webhook secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// getOwnerAndRepo extracts the GitHub owner and repo from a project's
// adapters configuration. Returns empty strings if no GitHub adapter is
// configured.
func getOwnerAndRepo(project model.Project) (owner, repo string) {
	for _, a := range project.Adapters {
		if a.Type == "github" {
			return a.Config["owner"], a.Config["repo"]
		}
	}
	return "", ""
}

// CreateForProject registers a webhook for the given project. It generates a
// secret, stores it, calls the GitHub API to create the webhook, and updates
// the project with the webhook ID. If GitHub is not configured or the
// FLUX_WEBHOOK_URL is not set, it logs a warning and returns nil (best-effort).
// Errors are logged but not returned (fire-and-forget semantics).
func (c *WebhookCreator) CreateForProject(ctx context.Context, project model.Project) {
	if c.appAuth == nil {
		slog.Warn("webhook creator: GitHub App not configured, skipping webhook registration",
			"project_id", project.ID)
		return
	}

	webhookURL := c.webhookURLFn()
	if webhookURL == "" {
		slog.Warn("webhook creator: FLUX_WEBHOOK_URL not set, skipping webhook registration",
			"project_id", project.ID)
		return
	}

	// Ensure the webhook URL ends with the correct path.
	if !strings.HasSuffix(webhookURL, "/api/v1/webhooks/github") {
		webhookURL = strings.TrimRight(webhookURL, "/") + "/api/v1/webhooks/github"
	}

	owner, repo := getOwnerAndRepo(project)
	if owner == "" || repo == "" {
		slog.Warn("webhook creator: no GitHub adapter config found, skipping webhook registration",
			"project_id", project.ID)
		return
	}

	secret, err := generateSecret()
	if err != nil {
		slog.Warn("webhook creator: failed to generate secret", "project_id", project.ID, "error", err)
		return
	}

	webhookID, err := github.RegisterWebhook(
		ctx,
		c.appAuth,
		project.InstallationID,
		owner,
		repo,
		webhookURL,
		secret,
	)
	if err != nil {
		slog.Warn("webhook creator: failed to register webhook with GitHub",
			"project_id", project.ID,
			"error", err,
		)
		return
	}

	// Store the secret.
	if err := c.secretRepo.Set(ctx, project.RepoURL, secret); err != nil {
		slog.Warn("webhook creator: failed to store webhook secret",
			"project_id", project.ID, "error", err)
		return
	}

	// Update the project with the webhook ID.
	project.WebhookID = webhookID
	if err := c.projectRepo.Update(ctx, project); err != nil {
		slog.Warn("webhook creator: failed to update project with webhook ID",
			"project_id", project.ID, "error", err)
		return
	}

	slog.Info("webhook creator: registered webhook for project",
		"project_id", project.ID,
		"webhook_id", webhookID,
		"owner", owner,
		"repo", repo,
	)
}
