import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { AuditLog } from './AuditLog';

// ---- Test wrapper ----

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
    },
  });

  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    );
  };
}

function renderAuditLog() {
  const wrapper = createWrapper();
  return render(<AuditLog />, { wrapper });
}

// ---- Helpers ----

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

// ---- Sample fixtures matching the AuditEvent domain model ----

const sampleEvents = [
  {
    id: 'aev-1',
    actor_id: 'user-1',
    action: 'project.created',
    resource_type: 'project',
    resource_id: 'proj-1',
    metadata: '{}',
    created_at: '2026-06-24T10:00:00Z',
  },
  {
    id: 'aev-2',
    actor_id: 'user-1',
    action: 'ticket.updated',
    resource_type: 'ticket',
    resource_id: 'ticket-42',
    metadata: '{}',
    created_at: '2026-06-24T09:00:00Z',
  },
  {
    id: 'aev-3',
    actor_id: 'user-2',
    action: 'project.created',
    resource_type: 'project',
    resource_id: 'proj-2',
    metadata: '{}',
    created_at: '2026-06-23T14:30:00Z',
  },
];

describe('AuditLog', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // --- Loading state ---

  it('shows loading skeleton while audit events are being fetched', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderAuditLog();

    const skeleton = screen.getByRole('status', { name: /loading/i });
    expect(skeleton).toBeInTheDocument();
  });

  it('does not show table while still loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderAuditLog();

    expect(screen.queryByRole('table')).toBeNull();
  });

  // --- Empty state ---

  it('shows an empty state message when there are no audit events', async () => {
    mockFetch.mockResolvedValue(jsonResponse([]));

    renderAuditLog();

    await waitFor(() => {
      expect(screen.getByText(/no audit events/i)).toBeInTheDocument();
    });
  });

  // --- Error state ---

  it('shows an error banner when the fetch fails', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: 'Internal Server Error' }, 500));

    renderAuditLog();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/internal server error/i)).toBeInTheDocument();
    });
  });

  it('shows an error banner on network failure', async () => {
    mockFetch.mockRejectedValue(new Error('Network Error'));

    renderAuditLog();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/network error/i)).toBeInTheDocument();
    });
  });

  // --- Access denied (403) ---

  it('shows "Access denied" message on 403 response', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: 'Forbidden' }, 403));

    renderAuditLog();

    await waitFor(() => {
      expect(screen.getByText(/access denied/i)).toBeInTheDocument();
    });
  });

  // --- Success state: table rendering ---

  it('renders audit events table when data is loaded', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleEvents));

    renderAuditLog();

    await waitFor(() => {
      const table = screen.getByRole('table');
      expect(table).toBeInTheDocument();
    });
  });

  it('renders correct column headers', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleEvents));

    renderAuditLog();

    await waitFor(() => {
      expect(screen.getByText(/actor/i)).toBeInTheDocument();
      expect(screen.getByText(/action/i)).toBeInTheDocument();
      expect(screen.getByText(/resource type/i)).toBeInTheDocument();
      expect(screen.getByText(/resource id/i)).toBeInTheDocument();
      expect(screen.getByText(/created at/i)).toBeInTheDocument();
    });
  });

  it('renders audit event data in table rows', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleEvents));

    renderAuditLog();

    await waitFor(() => {
      // Some values appear in multiple rows — use getAllByText
      expect(screen.getAllByText('user-1')).toHaveLength(2);
      expect(screen.getAllByText('project.created')).toHaveLength(2);
      expect(screen.getByText('ticket.updated')).toBeInTheDocument();
      expect(screen.getByText('proj-1')).toBeInTheDocument();
      expect(screen.getByText('ticket-42')).toBeInTheDocument();
      expect(screen.getByText('user-2')).toBeInTheDocument();
      expect(screen.getByText('proj-2')).toBeInTheDocument();
    });
  });

  it('renders the correct number of rows', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleEvents));

    renderAuditLog();

    await waitFor(() => {
      const rows = screen.getAllByRole('row');
      // 1 header row + 3 data rows
      expect(rows).toHaveLength(4);
    });
  });

  // --- Fetch URL ---

  it('fetches audit events from /api/v1/audit-events', async () => {
    mockFetch.mockResolvedValue(jsonResponse([]));

    renderAuditLog();

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/audit-events',
        expect.objectContaining({
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
          }),
        }),
      );
    });
  });
});
