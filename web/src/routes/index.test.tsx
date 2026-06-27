import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { Dashboard } from './index';

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

function renderDashboard() {
  const wrapper = createWrapper();
  return render(<Dashboard />, { wrapper });
}

// ---- Helpers ----

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('Dashboard', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // --- Loading state ---

  it('shows loading skeletons while fetching counts', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderDashboard();

    expect(
      screen.getByRole('status', { name: /loading/i }),
    ).toBeInTheDocument();
  });

  it('does not show error while loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderDashboard();

    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('does not show stat cards while loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderDashboard();

    expect(screen.queryByText(/projects/i)).toBeNull();
  });

  // --- Success state ---

  it('displays correct counts from all four endpoints', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/pipeline-runs')) {
        return Promise.resolve(
          jsonResponse({
            items: Array.from({ length: 7 }, (_, i) => ({ id: `${i}` })),
          }),
        );
      }
      if (url.includes('/pull-requests')) {
        return Promise.resolve(jsonResponse({ items: [{ id: '1' }, { id: '2' }, { id: '3' }] }));
      }
      if (url.includes('/tickets')) {
        return Promise.resolve(
          jsonResponse({
            items: Array.from({ length: 12 }, (_, i) => ({ id: `${i}` })),
          }),
        );
      }
      if (url.includes('/projects')) {
        return Promise.resolve(
          jsonResponse(
            Array.from({ length: 5 }, (_, i) => ({ id: `${i}` })),
          ),
        );
      }
      if (url.includes('/sync/status')) {
        return Promise.resolve(
          jsonResponse({
            last_sync_at: new Date().toISOString(),
            last_sync_error: '',
            tickets_synced: 5,
            prs_synced: 3,
            webhooks_healthy: true,
          }),
        );
      }
      return Promise.reject(new Error('Unknown URL'));
    });

    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText('Projects')).toBeInTheDocument();
      expect(screen.getByText('Tickets')).toBeInTheDocument();
      expect(screen.getByText('Pull Requests')).toBeInTheDocument();
      expect(screen.getByText('Pipeline Runs')).toBeInTheDocument();
    });

    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('12')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('7')).toBeInTheDocument();
  });

  it('renders StatCard links with correct hrefs', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/sync/status')) {
        return Promise.resolve(
          jsonResponse({
            last_sync_at: new Date().toISOString(),
            last_sync_error: '',
            tickets_synced: 0,
            prs_synced: 0,
            webhooks_healthy: true,
          }),
        );
      }
      return Promise.resolve(jsonResponse([]));
    });

    renderDashboard();

    await waitFor(() => {
      const links = screen.getAllByRole('link');
      const projectLink = links.find((l) => l.getAttribute('href') === '/projects');
      const ticketLink = links.find((l) => l.getAttribute('href') === '/tickets');
      const prLink = links.find((l) => l.getAttribute('href') === '/pull-requests');
      const pipelineLink = links.find((l) => l.getAttribute('href') === '/pipeline-runs');

      expect(projectLink).toBeInTheDocument();
      expect(ticketLink).toBeInTheDocument();
      expect(prLink).toBeInTheDocument();
      expect(pipelineLink).toBeInTheDocument();
    });
  });

  it('shows four StatCards on success', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/sync/status')) {
        return Promise.resolve(
          jsonResponse({
            last_sync_at: new Date().toISOString(),
            last_sync_error: '',
            tickets_synced: 0,
            prs_synced: 0,
            webhooks_healthy: true,
          }),
        );
      }
      return Promise.resolve(jsonResponse([]));
    });

    renderDashboard();

    await waitFor(() => {
      expect(screen.getAllByRole('link')).toHaveLength(4);
    });
  });

  // --- Empty state ---

  it('shows zero counts when all endpoints return empty arrays', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/sync/status')) {
        return Promise.resolve(
          jsonResponse({
            last_sync_at: new Date().toISOString(),
            last_sync_error: '',
            tickets_synced: 0,
            prs_synced: 0,
            webhooks_healthy: true,
          }),
        );
      }
      return Promise.resolve(jsonResponse([]));
    });

    renderDashboard();

    await waitFor(() => {
      const zeroes = screen.getAllByText('0');
      expect(zeroes).toHaveLength(4);
    });
  });

  // --- Error state ---

  it('shows an error banner when a fetch fails', async () => {
    mockFetch.mockRejectedValue(new Error('Failed to fetch'));

    renderDashboard();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/failed to fetch/i)).toBeInTheDocument();
    });
  });

  it('shows an error banner on 500 response', async () => {
    mockFetch.mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify({ error: 'Internal Server Error' }), {
          status: 500,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    );

    renderDashboard();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/internal server error/i)).toBeInTheDocument();
    });
  });

  it('does not show stat cards when in error state', async () => {
    mockFetch.mockRejectedValue(new Error('API Error'));

    renderDashboard();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    expect(screen.queryByRole('link')).toBeNull();
  });
});
