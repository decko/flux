import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { SyncStatusSection } from './SyncStatusSection';

/** Minimal test wrapper that provides TanStack Query with retries disabled. */
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

function renderSection() {
  const wrapper = createWrapper();
  return render(<SyncStatusSection />, { wrapper });
}

/** Helper to create a JSON response with the given body and status. */
function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('SyncStatusSection', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // --- Loading state ---

  it('renders loading state initially', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderSection();

    expect(screen.getByText(/loading sync status/i)).toBeInTheDocument();
  });

  // --- Success state ---

  it('renders sync status when data loads', async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({
        last_sync_at: new Date().toISOString(),
        last_sync_error: '',
        tickets_synced: 5,
        prs_synced: 3,
        webhooks_healthy: true,
      }),
    );

    renderSection();

    await waitFor(() => {
      expect(screen.getByText(/5 tickets/)).toBeInTheDocument();
      expect(screen.getByText(/3 PRs/)).toBeInTheDocument();
    });
  });

  it('shows "never" when last_sync_at is null', async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({
        last_sync_at: null,
        last_sync_error: '',
        tickets_synced: 0,
        prs_synced: 0,
        webhooks_healthy: true,
      }),
    );

    renderSection();

    await waitFor(() => {
      expect(screen.getByText(/never/i)).toBeInTheDocument();
    });
  });

  it('shows healthy when webhooks_healthy is true', async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({
        last_sync_at: new Date().toISOString(),
        last_sync_error: '',
        tickets_synced: 0,
        prs_synced: 0,
        webhooks_healthy: true,
      }),
    );

    renderSection();

    await waitFor(() => {
      expect(screen.getByText(/healthy/i)).toBeInTheDocument();
    });
  });

  it('shows "unhealthy" with red dot when webhooks_healthy is false', async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({
        last_sync_at: new Date().toISOString(),
        last_sync_error: '',
        tickets_synced: 0,
        prs_synced: 0,
        webhooks_healthy: false,
      }),
    );

    renderSection();

    await waitFor(() => {
      expect(screen.getByText(/unhealthy/i)).toBeInTheDocument();
    });
  });

  // --- Error state ---

  it('shows error state when fetch fails', async () => {
    mockFetch.mockRejectedValue(new Error('Network error'));

    renderSection();

    await waitFor(() => {
      expect(screen.getByText(/network error/i)).toBeInTheDocument();
    });
  });

  it('shows retry button on error', async () => {
    mockFetch.mockRejectedValue(new Error('Network error'));

    renderSection();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
    });
  });

  it('retry button refetches data', async () => {
    const user = userEvent.setup();

    // First call fails, second call succeeds
    mockFetch
      .mockRejectedValueOnce(new Error('Network error'))
      .mockResolvedValueOnce(
        jsonResponse({
          last_sync_at: new Date().toISOString(),
          last_sync_error: '',
          tickets_synced: 10,
          prs_synced: 2,
          webhooks_healthy: true,
        }),
      );

    renderSection();

    // Wait for error state
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /retry/i }));

    // After retry, should show success state with data
    await waitFor(() => {
      expect(screen.getByText(/10 tickets/)).toBeInTheDocument();
    });
  });
});
