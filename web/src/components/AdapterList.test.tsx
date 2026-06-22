import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { AdapterList } from './AdapterList';

/** Minimal test wrapper that provides TanStack Query with retries disabled. */
function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
      mutations: {
        retry: false,
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

function renderList() {
  const wrapper = createWrapper();
  return render(<AdapterList />, { wrapper });
}

// ---- Helpers for building mock fetch responses ----

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function jsonErrorResponse(status: number, message: string): Response {
  return new Response(JSON.stringify({ error: message }), {
    status,
    statusText: message,
    headers: { 'Content-Type': 'application/json' },
  });
}

// ---- Sample fixtures matching #48 API contract ----

const sampleAdapters = [
  { type: 'github', name: 'flux-org/flux-core', health: 'healthy' },
  { type: 'jira', name: 'flux-org/flux-web', health: 'unhealthy' },
];

const sampleSyncStatus = {
  lastSyncAt: '2026-06-21T14:30:00Z',
  lastSyncError: '',
  ticketsSynced: 42,
  prsSynced: 7,
};

const syncStatusWithError = {
  lastSyncAt: '2026-06-20T08:00:00Z',
  lastSyncError: 'Jira API authentication failed',
  ticketsSynced: 15,
  prsSynced: 0,
};

describe('AdapterList', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // --- Loading state ---

  it('shows loading skeletons while adapter data is being fetched', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderList();

    const skeletons = screen.getByRole('status', { name: /loading/i });
    expect(skeletons).toBeInTheDocument();
  });

  it('shows loading skeletons while sync status is being fetched', () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/api/v1/adapters')) {
        return jsonResponse(sampleAdapters);
      }
      return new Promise(() => {});
    });

    renderList();

    expect(screen.queryByTestId('adapter-card')).toBeNull();
    const skeletons = screen.getByRole('status', { name: /loading/i });
    expect(skeletons).toBeInTheDocument();
  });

  // --- Empty state ---

  it('shows an empty state message when no adapters are configured', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/api/v1/adapters')) {
        return jsonResponse([]);
      }
      if (url.includes('/api/v1/sync/status')) {
        return jsonResponse(sampleSyncStatus);
      }
      return jsonErrorResponse(404, 'Not found');
    });

    renderList();

    await waitFor(() => {
      expect(
        screen.getByText(/no adapters configured/i),
      ).toBeInTheDocument();
    });
  });

  it('does not show empty state while still loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderList();

    expect(screen.queryByText(/no adapters configured/i)).toBeNull();
  });

  // --- Error state ---

  it('shows an error banner when the adapters fetch fails', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/api/v1/adapters')) {
        return jsonErrorResponse(500, 'Internal Server Error');
      }
      return jsonResponse(sampleSyncStatus);
    });

    renderList();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/internal server error/i)).toBeInTheDocument();
    });
  });

  it('shows an error banner when the sync status fetch fails', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/api/v1/adapters')) {
        return jsonResponse(sampleAdapters);
      }
      return jsonErrorResponse(500, 'Sync unavailable');
    });

    renderList();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/sync unavailable/i)).toBeInTheDocument();
    });
  });

  it('shows a retry button in the error banner', async () => {
    mockFetch
      .mockResolvedValueOnce(jsonErrorResponse(500, 'boom'))
      .mockResolvedValueOnce(jsonResponse(sampleSyncStatus))
      .mockResolvedValueOnce(jsonResponse(sampleAdapters));

    const user = userEvent.setup();
    renderList();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    const retryButton = screen.getByRole('button', { name: /retry/i });
    expect(retryButton).toBeInTheDocument();

    await user.click(retryButton);

    await waitFor(() => {
      expect(screen.getByText(/flux-org\/flux-core/)).toBeInTheDocument();
    });
  });

  // --- Success state ---

  it('renders adapter cards when data is loaded', async () => {
    mockFetch
      .mockResolvedValueOnce(jsonResponse(sampleAdapters))
      .mockResolvedValueOnce(jsonResponse(sampleSyncStatus));

    renderList();

    await waitFor(() => {
      expect(screen.getByText(/flux-org\/flux-core/)).toBeInTheDocument();
      expect(screen.getByText(/flux-org\/flux-web/)).toBeInTheDocument();
    });
  });

  it('renders one card per adapter', async () => {
    mockFetch
      .mockResolvedValueOnce(jsonResponse(sampleAdapters))
      .mockResolvedValueOnce(jsonResponse(sampleSyncStatus));

    renderList();

    await waitFor(() => {
      const cards = screen.getAllByTestId('adapter-card');
      expect(cards).toHaveLength(sampleAdapters.length);
    });
  });

  // --- Global sync status ---

  it('shows global ticket and PR counts from sync status', async () => {
    mockFetch
      .mockResolvedValueOnce(jsonResponse(sampleAdapters))
      .mockResolvedValueOnce(jsonResponse(sampleSyncStatus));

    renderList();

    await waitFor(() => {
      expect(screen.getByText(/42/)).toBeInTheDocument();
      expect(screen.getByText(/7/)).toBeInTheDocument();
    });
  });

  it('shows the last sync time from global sync status', async () => {
    mockFetch
      .mockResolvedValueOnce(jsonResponse(sampleAdapters))
      .mockResolvedValueOnce(jsonResponse(sampleSyncStatus));

    renderList();

    await waitFor(() => {
      expect(screen.getByText(/last sync/i)).toBeInTheDocument();
    });
  });

  // --- Global sync error ---

  it('shows a global sync error banner when sync has an error', async () => {
    mockFetch
      .mockResolvedValueOnce(jsonResponse(sampleAdapters))
      .mockResolvedValueOnce(jsonResponse(syncStatusWithError));

    renderList();

    await waitFor(() => {
      expect(
        screen.getByText(/jira api authentication failed/i),
      ).toBeInTheDocument();
    });
  });

  it('does not show sync error when lastSyncError is empty', async () => {
    mockFetch
      .mockResolvedValueOnce(jsonResponse(sampleAdapters))
      .mockResolvedValueOnce(jsonResponse(sampleSyncStatus));

    renderList();

    await waitFor(() => {
      expect(screen.getByText(/flux-org\/flux-core/)).toBeInTheDocument();
    });

    expect(
      screen.queryByRole('alert'),
    ).not.toBeInTheDocument();
  });

  // --- Global Sync Now ---

  it('triggers a global sync when Sync Now is clicked', async () => {
    const user = userEvent.setup();
    mockFetch
      .mockResolvedValueOnce(jsonResponse(sampleAdapters))
      .mockResolvedValueOnce(jsonResponse(sampleSyncStatus))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ ok: true }), { status: 202 }),
      );

    renderList();

    await waitFor(() => {
      expect(screen.getByText(/flux-org\/flux-core/)).toBeInTheDocument();
    });

    const syncButton = screen.getByRole('button', { name: /sync now/i });
    await user.click(syncButton);

    // The third call should be the POST to sync/trigger
    expect(mockFetch).toHaveBeenNthCalledWith(
      3,
      '/api/v1/sync/trigger',
      expect.objectContaining({ method: 'POST' }),
    );
    // Body should be absent (global sync)
    const syncCallArgs = mockFetch.mock.calls[2];
    if (!syncCallArgs) return;
    if (syncCallArgs[1] && typeof syncCallArgs[1] === 'object' && 'body' in syncCallArgs[1]) {
      expect((syncCallArgs[1] as Record<string, unknown>).body).toBeUndefined();
    }
  });

  it('shows a spinner on Sync Now while the sync is in progress', async () => {
    const user = userEvent.setup();
    let resolveSync!: (value: Response) => void;
    mockFetch
      .mockResolvedValueOnce(jsonResponse(sampleAdapters))
      .mockResolvedValueOnce(jsonResponse(sampleSyncStatus))
      .mockReturnValueOnce(
        new Promise<Response>((r) => {
          resolveSync = r;
        }),
      );

    renderList();

    await waitFor(() => {
      expect(screen.getByText(/flux-org\/flux-core/)).toBeInTheDocument();
    });

    const syncButton = screen.getByRole('button', { name: /sync now/i });
    await user.click(syncButton);

    expect(screen.getByLabelText(/syncing/i)).toBeInTheDocument();

    resolveSync(new Response(JSON.stringify({ ok: true }), { status: 202 }));

    await waitFor(() => {
      expect(screen.queryByLabelText(/syncing/i)).not.toBeInTheDocument();
    });
  });

  // --- Concurrent fetch behavior ---

  it('fetches adapters and sync status in parallel', async () => {
    mockFetch
      .mockResolvedValueOnce(jsonResponse(sampleAdapters))
      .mockResolvedValueOnce(jsonResponse(sampleSyncStatus));

    renderList();

    await waitFor(() => {
      expect(screen.getByText(/flux-org\/flux-core/)).toBeInTheDocument();
    });

    const adapterCall = mockFetch.mock.calls.find(
      (c: unknown[]) => c[0]?.toString().includes('/api/v1/adapters'),
    );
    const statusCall = mockFetch.mock.calls.find(
      (c: unknown[]) => c[0]?.toString().includes('/api/v1/sync/status'),
    );
    expect(adapterCall).toBeDefined();
    expect(statusCall).toBeDefined();
  });
});
