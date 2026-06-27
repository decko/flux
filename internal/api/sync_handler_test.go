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
