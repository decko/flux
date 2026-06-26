package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// githubEventType is the set of GitHub webhook event types that flux handles.
type githubEventType string

const (
	eventIssues      githubEventType = "issues"
	eventPullRequest githubEventType = "pull_request"
	eventPush        githubEventType = "push"
	eventPing        githubEventType = "ping"
)

// supportedEvents contains all GitHub event types that flux can process.
var supportedEvents = map[githubEventType]bool{
	eventIssues:      true,
	eventPullRequest: true,
	eventPush:        true,
	eventPing:        true,
}

// gitHubWebhookPayload is the minimal set of fields extracted from a GitHub
// webhook JSON payload for processing.
type gitHubWebhookPayload struct {
	Action     string              `json:"action"`
	Issue      *gitHubIssuePayload `json:"issue,omitempty"`
	PR         *gitHubPRPayload    `json:"pull_request,omitempty"`
	Repository gitHubRepoPayload   `json:"repository"`
	Sender     gitHubSenderPayload `json:"sender"`
	Label      *gitHubLabelPayload `json:"label,omitempty"`
}

type gitHubIssuePayload struct {
	Number  int               `json:"number"`
	Title   string            `json:"title"`
	State   string            `json:"state"`
	Labels  []gitHubLabelMeta `json:"labels"`
	HTMLURL string            `json:"html_url"`
}

type gitHubPRPayload struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	HTMLURL string `json:"html_url"`
}

type gitHubRepoPayload struct {
	FullName string `json:"full_name"`
}

type gitHubSenderPayload struct {
	Login string `json:"login"`
}

type gitHubLabelMeta struct {
	Name string `json:"name"`
}

type gitHubLabelPayload struct {
	Name string `json:"name"`
}

// botLogins is the set of bot account logins that should not trigger pipeline
// runs, even if they match a trigger rule.
var botLogins = map[string]bool{
	"flux-bot":            true,
	"github-actions[bot]": true,
	"renovate[bot]":       true,
	"dependabot[bot]":     true,
}

// isBotSender returns true if the sender login corresponds to a known bot
// account.
func isBotSender(login string) bool {
	return botLogins[login]
}

// repoURLFromFullName constructs a GitHub repo URL from a full name
// (e.g., "owner/repo" → "https://github.com/owner/repo").
func repoURLFromFullName(fullName string) string {
	return fmt.Sprintf("https://github.com/%s", fullName)
}

// handleGitHubWebhook processes incoming GitHub webhook requests. It verifies
// the HMAC-SHA256 signature, parses the event type, extracts ticket/pull
// request data from the payload, upserts tickets, and triggers pipeline runs
// when matching trigger rules exist.
func (s *Server) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	// Limit body size to 5MB to prevent abuse.
	body, err := io.ReadAll(io.LimitReader(r.Body, 5<<20))
	if err != nil {
		slog.Warn("webhook: failed to read body", "error", err)
		writeJSONError(w, http.StatusBadRequest, "failed to read request body", "")
		return
	}
	defer func() { _ = r.Body.Close() }()

	// Verify HMAC signature.
	sig := r.Header.Get("X-Hub-Signature-256")
	if sig == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing signature", "")
		return
	}

	// Parse the payload to extract the repo URL (needed to look up the secret).
	var payload gitHubWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Warn("webhook: invalid JSON payload", "error", err)
		writeJSONError(w, http.StatusBadRequest, "invalid JSON payload", "")
		return
	}

	repoURL := repoURLFromFullName(payload.Repository.FullName)

	// Look up the webhook secret for this repo.
	secret, err := s.webhookSecretRepo.Get(r.Context(), repoURL)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// No secret configured for this repo — accept the event but
			// don't process it (the repo may not be managed by flux).
			slog.Info("webhook: no secret found for repo, skipping", "repo", repoURL)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
			return
		}
		slog.Error("webhook: failed to get secret", "repo", repoURL, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error", "")
		return
	}

	if !validSignature(body, sig, secret) {
		writeJSONError(w, http.StatusUnauthorized, "invalid signature", "")
		return
	}

	// Parse the event type.
	eventType := githubEventType(r.Header.Get("X-GitHub-Event"))
	if !supportedEvents[eventType] {
		slog.Debug("webhook: unsupported event type", "event", eventType)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ignored"})
		return
	}

	// Handle ping events (no processing needed).
	if eventType == eventPing {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "pong"})
		return
	}

	// If core services aren't wired, skip processing gracefully.
	if s.projectSvc == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Find the project by repo URL.
	projects, err := s.projectSvc.List(r.Context(), repository.ProjectFilter{})
	if err != nil {
		slog.Error("webhook: failed to list projects", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error", "")
		return
	}

	var project *model.Project
	for i := range projects {
		if projects[i].RepoURL == repoURL {
			project = &projects[i]
			break
		}
	}
	if project == nil {
		// No project found for this repo — accept but don't process.
		slog.Info("webhook: no project found for repo, skipping", "repo", repoURL)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
		return
	}

	// Dispatch based on event type.
	switch eventType {
	case eventIssues:
		s.handleIssueEvent(w, r, project, payload)
	case eventPullRequest:
		s.handlePREvent(w, r, project, payload)
	case eventPush:
		s.handlePushEvent(w, r, project)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ignored"})
	}
}

// handleIssueEvent processes an issues webhook event. It upserts a ticket
// and evaluates trigger rules for labeled events.
func (s *Server) handleIssueEvent(w http.ResponseWriter, r *http.Request, project *model.Project, payload gitHubWebhookPayload) {
	if payload.Issue == nil {
		slog.Warn("webhook: issues event missing issue data")
		writeJSONError(w, http.StatusBadRequest, "malformed payload", "")
		return
	}

	ticket := issueToTicket(project.ID, payload)

	if err := s.ticketSvc.Create(r.Context(), ticket); err != nil {
		// If the ticket already exists, update it.
		if _, err2 := s.ticketSvc.Get(r.Context(), ticket.ID); err2 == nil {
			if err := s.ticketSvc.Update(r.Context(), ticket); err != nil {
				slog.Error("webhook: failed to upsert ticket", "error", err)
				writeJSONError(w, http.StatusInternalServerError, "internal error", "")
				return
			}
		} else {
			slog.Error("webhook: failed to create ticket", "error", err)
			writeJSONError(w, http.StatusInternalServerError, "internal error", "")
			return
		}
	}

	// Only trigger pipeline runs for labeled events from non-bot senders.
	if payload.Action == "labeled" && !isBotSender(payload.Sender.Login) {
		// Evaluate trigger rules for the project.
		if s.triggerRuleRepo != nil {
			s.triggerForTicket(r.Context(), project, ticket, "ticket.labeled")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "created"})
}

// handlePREvent processes a pull_request webhook event. It upserts a pull
// request record.
func (s *Server) handlePREvent(w http.ResponseWriter, r *http.Request, project *model.Project, payload gitHubWebhookPayload) {
	if payload.PR == nil {
		slog.Warn("webhook: pull_request event missing PR data")
		writeJSONError(w, http.StatusBadRequest, "malformed payload", "")
		return
	}

	pr := prToPullRequest(project.ID, payload)

	// Upsert via the PR service.
	if err := s.prSvc.Create(r.Context(), pr); err != nil {
		if _, err2 := s.prSvc.Get(r.Context(), pr.ID); err2 == nil {
			if err := s.prSvc.Update(r.Context(), pr); err != nil {
				slog.Error("webhook: failed to upsert pull request", "error", err)
				writeJSONError(w, http.StatusInternalServerError, "internal error", "")
				return
			}
		} else {
			slog.Error("webhook: failed to create pull request", "error", err)
			writeJSONError(w, http.StatusInternalServerError, "internal error", "")
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "created"})
}

// handlePushEvent processes a push webhook event. Currently this is a no-op
// placeholder for future automation (e.g., triggering CI pipelines on push).
func (s *Server) handlePushEvent(w http.ResponseWriter, r *http.Request, project *model.Project) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// triggerForTicket evaluates trigger rules for the given project and ticket
// for the specified event type. If a matching rule is found, a new pipeline
// run is created. Rules with an empty event match any event type (backward
// compatibility).
func (s *Server) triggerForTicket(ctx context.Context, project *model.Project, ticket model.Ticket, eventType string) {
	if s.triggerRuleRepo == nil || s.pipelineSvc == nil {
		return
	}
	rules, err := s.triggerRuleRepo.ListByProject(ctx, project.ID)
	if err != nil {
		slog.Error("webhook: failed to list trigger rules", "error", err, "project_id", project.ID)
		return
	}
	if len(rules) == 0 {
		return
	}

	// Find the highest-priority enabled rule whose label matches the ticket
	// and whose event matches the given eventType.
	var bestPipeline string
	bestPriority := -1
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if !model.EventMatchesRule(eventType, rule.Event) {
			continue
		}
		if hasLabel(ticket.Labels, rule.Label) && rule.Priority > bestPriority {
			bestPriority = rule.Priority
			bestPipeline = rule.Pipeline
		}
	}

	if bestPipeline == "" {
		return
	}

	// Validate the pipeline exists in the project config.
	var found bool
	for _, p := range project.Pipelines {
		if p.Name == bestPipeline {
			found = true
			break
		}
	}
	if !found {
		slog.Info("webhook: pipeline not configured for project, skipping",
			"pipeline", bestPipeline, "project_id", project.ID)
		return
	}

	run := model.PipelineRun{
		ID:           uuid.New().String(),
		ProjectID:    project.ID,
		TicketID:     ticket.ID,
		Orchestrator: "soda",
		Pipeline:     bestPipeline,
		Status:       model.RunStatusPending,
		Phases:       []model.PhaseResult{},
	}
	if err := s.pipelineSvc.Create(ctx, run); err != nil {
		slog.Error("webhook: failed to create pipeline run", "error", err)
		return
	}

	slog.Info("webhook: triggered pipeline run",
		"ticket_id", ticket.ID, "pipeline", bestPipeline, "project_id", project.ID)
}

// hasLabel checks if the given label exists in the label slice.
func hasLabel(labels []string, target string) bool {
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}

// issueToTicket converts a GitHub issues webhook payload to a model.Ticket.
func issueToTicket(projectID string, payload gitHubWebhookPayload) model.Ticket {
	state := model.TicketStatusOpen
	if payload.Issue.State == "closed" {
		state = model.TicketStatusClosed
	}

	labels := make([]string, len(payload.Issue.Labels))
	for i, l := range payload.Issue.Labels {
		labels[i] = l.Name
	}

	return model.Ticket{
		ID:         externalIDToTicketID(fmt.Sprintf("github-%d", payload.Issue.Number)),
		ProjectID:  projectID,
		ExternalID: fmt.Sprintf("%d", payload.Issue.Number),
		Source:     model.TicketSourceGitHub,
		Title:      payload.Issue.Title,
		Status:     state,
		Labels:     labels,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
}

// prToPullRequest converts a GitHub pull_request webhook payload to a
// model.PullRequest.
func prToPullRequest(projectID string, payload gitHubWebhookPayload) model.PullRequest {
	state := model.PRStatusOpen
	if payload.PR.State == "closed" {
		state = model.PRStatusClosed
	}

	return model.PullRequest{
		ID:         externalIDToTicketID(fmt.Sprintf("github-pr-%d", payload.PR.Number)),
		ProjectID:  projectID,
		ExternalID: fmt.Sprintf("%d", payload.PR.Number),
		Source:     model.PRSourceGitHub,
		Title:      payload.PR.Title,
		Status:     state,
		URL:        payload.PR.HTMLURL,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
}

// externalIDToTicketID generates a deterministic ID from an external
// identifier. It uses a UUID v5 with a namespace for deterministic mapping.
func externalIDToTicketID(externalID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(externalID)).String()
}

// validSignature checks whether the given HMAC-SHA256 signature matches the
// body payload signed with the given secret. The expected format is
// "sha256=<hex-digest>", matching GitHub's X-Hub-Signature-256 header.
func validSignature(body []byte, signature, secret string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	expected := signature[7:] // strip "sha256=" prefix
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	got := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(got), []byte(expected))
}
