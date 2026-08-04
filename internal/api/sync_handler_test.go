package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/decko/flux/internal/domain"
)

// mockSyncService is a thread-safe in-memory SyncService for testing.
type mockSyncService struct {
	mu            sync.Mutex
	lastSyncAt    *time.Time
	lastSyncError string
	ticketsSynced int
	prsSynced     int
	projects      map[string]domain.ProjectSyncStatus
}

func (s *mockSyncService) Status() domain.SyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return domain.SyncStatus{
		LastSyncAt:      s.lastSyncAt,
		LastSyncError:   s.lastSyncError,
		TicketsSynced:   s.ticketsSynced,
		PRsSynced:       s.prsSynced,
		WebhooksHealthy: true,
		Projects:        s.projects,
	}
}

func (s *mockSyncService) SyncNow(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.lastSyncAt = &now
	s.lastSyncError = ""
	s.ticketsSynced = 5
	s.prsSynced = 3
	return nil
}

func (s *mockSyncService) SyncProject(_ context.Context, _ string) error {
	return nil
}

// setupSyncServer creates a Server with a mock SyncService for testing sync endpoints.
func setupSyncServer(t *testing.T) (*Server, *mockSyncService) {
	t.Helper()
	svc := &mockSyncService{}
	srv := NewServer(WithJWTSecret(testJWTSecretBytes), WithSyncService(svc))
	return srv, svc
}

// ─── Sync Status ──────────────────────────────────────────────────────────

func TestHandleSyncStatus(t *testing.T) {
	srv, _ := setupSyncServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/sync/status", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/sync/status: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var status syncStatusResponse
	mustDecode(t, resp, &status)

	if status.LastSyncAt != nil {
		t.Error("expected nil last_sync_at")
	}
	if status.LastSyncError != "" {
		t.Errorf("got last_sync_error %q, want ''", status.LastSyncError)
	}
	if status.TicketsSynced != 0 {
		t.Errorf("got tickets_synced %d, want 0", status.TicketsSynced)
	}
	if status.PRsSynced != 0 {
		t.Errorf("got prs_synced %d, want 0", status.PRsSynced)
	}
}

// ─── Sync Trigger ─────────────────────────────────────────────────────────

func TestHandleSyncTrigger(t *testing.T) {
	srv, _ := setupSyncServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/sync/trigger", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/sync/trigger: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
}

// ─── Sync Conflict (already in progress) ──────────────────────────────────

func TestHandleSyncTrigger_Conflict(t *testing.T) {
	srv, _ := setupSyncServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Lock the mutex to simulate an in-progress sync.
	srv.syncMu.Lock()
	defer srv.syncMu.Unlock()

	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/sync/trigger", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/sync/trigger (conflict): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

// ─── Sync Status After Trigger ────────────────────────────────────────────

func TestHandleSyncStatus_AfterTrigger(t *testing.T) {
	srv, _ := setupSyncServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Trigger sync.
	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/sync/trigger", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/sync/trigger: %v", err)
	}
	_ = resp.Body.Close()

	// Poll status until sync completes (async goroutine).
	var status syncStatusResponse
	for i := 0; i < 50; i++ {
		req = authedRequest(http.MethodGet, ts.URL+"/api/v1/sync/status", nil)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/sync/status: %v", err)
		}

		mustDecode(t, resp, &status)
		_ = resp.Body.Close()

		if status.LastSyncAt != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if status.LastSyncAt == nil {
		t.Error("expected non-nil last_sync_at after trigger")
	}
	if status.TicketsSynced <= 0 {
		t.Errorf("got tickets_synced %d, want > 0", status.TicketsSynced)
	}
	if status.PRsSynced <= 0 {
		t.Errorf("got prs_synced %d, want > 0", status.PRsSynced)
	}
}

func TestHandleSyncStatus_WebhooksHealthyField(t *testing.T) {
	srv, _ := setupSyncServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/sync/status", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/sync/status: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var status syncStatusResponse
	mustDecode(t, resp, &status)

	if !status.WebhooksHealthy {
		t.Error("expected webhooks_healthy to be true by default")
	}
}

// ─── Sync Service Not Configured ──────────────────────────────────────────

func TestHandleSyncStatus_ServiceNotConfigured(t *testing.T) {
	srv := NewServer(WithJWTSecret(testJWTSecretBytes))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/sync/status", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/sync/status: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestHandleSyncTrigger_ServiceNotConfigured(t *testing.T) {
	srv := NewServer(WithJWTSecret(testJWTSecretBytes))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/sync/trigger", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/sync/trigger: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

// ─── Unauthorized ─────────────────────────────────────────────────────────

func TestHandleSyncStatus_Unauthorized(t *testing.T) {
	srv, _ := setupSyncServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/api/v1/sync/status", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/sync/status (no auth): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestHandleSyncTrigger_Unauthorized(t *testing.T) {
	srv, _ := setupSyncServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/sync/trigger", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/sync/trigger (no auth): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestHandleSyncTrigger_Forbidden(t *testing.T) {
	srv, _ := setupSyncServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := nonAdminRequest(http.MethodPost, ts.URL+"/api/v1/sync/trigger", "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/sync/trigger (non-admin): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// ─── Per-Project Sync Status (#284) ────────────────────────────────────────

// TestSyncStatus_IncludesPerProjectData verifies GET /api/v1/sync/status
// includes per-project data under the "projects" key.
func TestSyncStatus_IncludesPerProjectData(t *testing.T) {
	srv, svc := setupSyncServer(t)
	now := time.Now().UTC()
	svc.projects = map[string]domain.ProjectSyncStatus{
		"proj-1": {
			ProjectID:       "proj-1",
			LastSyncAt:      &now,
			LastSyncError:   "",
			TicketsSynced:   2,
			PRsSynced:       1,
			WebhooksHealthy: true,
		},
		"proj-2": {
			ProjectID:       "proj-2",
			LastSyncAt:      &now,
			LastSyncError:   "ticket API error",
			TicketsSynced:   0,
			PRsSynced:       0,
			WebhooksHealthy: false,
		},
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/sync/status", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/sync/status: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var status syncStatusResponse
	mustDecode(t, resp, &status)

	if len(status.Projects) != 2 {
		t.Fatalf("got %d projects, want 2", len(status.Projects))
	}

	p1, ok := status.Projects["proj-1"]
	if !ok {
		t.Fatal("expected proj-1 in projects")
	}
	if p1.TicketsSynced != 2 {
		t.Errorf("proj-1 tickets_synced: got %d, want 2", p1.TicketsSynced)
	}
	if p1.PRsSynced != 1 {
		t.Errorf("proj-1 prs_synced: got %d, want 1", p1.PRsSynced)
	}
	if p1.LastSyncAt == nil {
		t.Error("proj-1: expected non-nil last_sync_at")
	}
	if !p1.WebhooksHealthy {
		t.Error("proj-1: expected webhooks_healthy true")
	}

	p2, ok := status.Projects["proj-2"]
	if !ok {
		t.Fatal("expected proj-2 in projects")
	}
	if p2.TicketsSynced != 0 {
		t.Errorf("proj-2 tickets_synced: got %d, want 0", p2.TicketsSynced)
	}
	if p2.PRsSynced != 0 {
		t.Errorf("proj-2 prs_synced: got %d, want 0", p2.PRsSynced)
	}
	if p2.WebhooksHealthy {
		t.Error("proj-2: expected webhooks_healthy false")
	}
}

// TestSyncStatus_PerProjectErrorAdminOnly verifies per-project last_sync_error
// is only revealed to admins, mirroring the aggregate error field behavior.
func TestSyncStatus_PerProjectErrorAdminOnly(t *testing.T) {
	srv, svc := setupSyncServer(t)
	now := time.Now().UTC()
	secretErr := "ticket API unavailable"
	svc.projects = map[string]domain.ProjectSyncStatus{
		"proj-1": {
			ProjectID:       "proj-1",
			LastSyncAt:      &now,
			LastSyncError:   secretErr,
			TicketsSynced:   0,
			PRsSynced:       0,
			WebhooksHealthy: true,
		},
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Non-admin: per-project error must be blanked out.
	req := nonAdminRequest(http.MethodGet, ts.URL+"/api/v1/sync/status", "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/sync/status (non-admin): %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var nonAdminStatus syncStatusResponse
	mustDecode(t, resp, &nonAdminStatus)
	_ = resp.Body.Close()
	p1, ok := nonAdminStatus.Projects["proj-1"]
	if !ok {
		t.Fatal("expected proj-1 in non-admin response")
	}
	if p1.LastSyncError != "" {
		t.Errorf("non-admin got last_sync_error %q, want ''", p1.LastSyncError)
	}

	// Admin: per-project error must be visible.
	req = authedRequest(http.MethodGet, ts.URL+"/api/v1/sync/status", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/sync/status (admin): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var adminStatus syncStatusResponse
	mustDecode(t, resp, &adminStatus)
	p1, ok = adminStatus.Projects["proj-1"]
	if !ok {
		t.Fatal("expected proj-1 in admin response")
	}
	if p1.LastSyncError != secretErr {
		t.Errorf("admin got last_sync_error %q, want %q", p1.LastSyncError, secretErr)
	}
}
