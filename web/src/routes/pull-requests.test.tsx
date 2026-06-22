import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { RouterProvider } from '@tanstack/react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act } from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createAppRouter } from '../router';
import { createMemoryHistory } from '@tanstack/react-router';

// ─── Helpers ───────────────────────────────────────────────────────────────

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

async function renderAtPath(initialPath: string) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  const memoryHistory = createMemoryHistory({ initialEntries: [initialPath] });
  const router = createAppRouter(memoryHistory);

  await act(async () => {
    await router.load();
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

// ─── Sample PR fixtures matching the API contract ─────────────────────────

const samplePRs = [
  {
    id: 'pr-1',
    project_id: 'proj-1',
    external_id: '42',
    source: 'github',
    title: 'Add user authentication',
    url: 'https://github.com/example/repo/pull/42',
    status: 'open',
    ticket_ids: ['ticket-1', 'ticket-2'],
    reviews: [
      { author: 'alice', status: 'approved', comment: 'LGTM', created_at: '2026-06-21T10:00:00Z' },
    ],
    created_at: '2026-06-20T08:00:00Z',
    updated_at: '2026-06-21T10:00:00Z',
  },
  {
    id: 'pr-2',
    project_id: 'proj-1',
    external_id: '43',
    source: 'github',
    title: 'Fix database migration',
    url: 'https://github.com/example/repo/pull/43',
    status: 'merged',
    ticket_ids: ['ticket-3'],
    reviews: [
      { author: 'bob', status: 'approved', comment: 'Looks good', created_at: '2026-06-19T14:00:00Z' },
      { author: 'carol', status: 'commented', comment: 'Nits', created_at: '2026-06-19T15:00:00Z' },
    ],
    created_at: '2026-06-18T08:00:00Z',
    updated_at: '2026-06-19T15:00:00Z',
  },
  {
    id: 'pr-3',
    project_id: 'proj-2',
    external_id: '44',
    source: 'gitlab',
    title: 'Update API docs',
    url: 'https://gitlab.com/example/repo/merge/44',
    status: 'closed',
    ticket_ids: [],
    reviews: [],
    created_at: '2026-06-17T08:00:00Z',
    updated_at: '2026-06-18T08:00:00Z',
  },
];

describe('PullRequestsPage', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // ─── Loading state ───────────────────────────────────────────────────

  it('shows a loading indicator while pull requests are being fetched', async () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    await renderAtPath('/pull-requests');

    expect(screen.getByRole('status', { name: /loading/i })).toBeInTheDocument();
  });

  // ─── Empty state ─────────────────────────────────────────────────────

  it('shows an empty state when no pull requests exist', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ items: [] }));

    renderAtPath('/pull-requests');

    await waitFor(() => {
      expect(screen.getByText(/no pull requests/i)).toBeInTheDocument();
    });
  });

  it('does not show empty state while still loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderAtPath('/pull-requests');

    expect(screen.queryByText(/no pull requests/i)).toBeNull();
  });

  // ─── Error state ─────────────────────────────────────────────────────

  it('shows an error banner when the fetch fails', async () => {
    mockFetch.mockResolvedValue(jsonErrorResponse(500, 'Internal Server Error'));

    renderAtPath('/pull-requests');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/internal server error/i)).toBeInTheDocument();
    });
  });

  it('shows a retry button in the error banner', async () => {
    mockFetch
      .mockResolvedValueOnce(jsonErrorResponse(500, 'boom'))
      .mockResolvedValueOnce(jsonResponse({ items: samplePRs }));

    const user = userEvent.setup();
    renderAtPath('/pull-requests');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    const retryButton = screen.getByRole('button', { name: /retry/i });
    expect(retryButton).toBeInTheDocument();

    await user.click(retryButton);

    await waitFor(() => {
      expect(screen.getByText('Add user authentication')).toBeInTheDocument();
    });
  });

  // ─── Success state ───────────────────────────────────────────────────

  it('renders PR titles from the API response', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ items: samplePRs }));

    renderAtPath('/pull-requests');

    await waitFor(() => {
      expect(screen.getByText('Add user authentication')).toBeInTheDocument();
      expect(screen.getByText('Fix database migration')).toBeInTheDocument();
      expect(screen.getByText('Update API docs')).toBeInTheDocument();
    });
  });

  it('renders status badges with correct labels', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ items: samplePRs }));

    renderAtPath('/pull-requests');

    await waitFor(() => {
      expect(screen.getByText('open')).toBeInTheDocument();
      expect(screen.getByText('merged')).toBeInTheDocument();
      expect(screen.getByText('closed')).toBeInTheDocument();
    });
  });

  it('renders PR URLs as clickable links', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ items: samplePRs }));

    renderAtPath('/pull-requests');

    await waitFor(() => {
      const links = screen.getAllByRole('link', { name: /view on github/i });
      expect(links[0]).toHaveAttribute(
        'href',
        'https://github.com/example/repo/pull/42',
      );
    });
  });

  it('shows the count of linked tickets for each PR', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ items: samplePRs }));

    renderAtPath('/pull-requests');

    await waitFor(() => {
      expect(screen.getByText(/2 tickets?/i)).toBeInTheDocument();
      expect(screen.getByText(/1 ticket?/i)).toBeInTheDocument();
    });
  });

  it('shows the count of reviews for each PR', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ items: samplePRs }));

    renderAtPath('/pull-requests');

    await waitFor(() => {
      expect(screen.getByText(/1 review/i)).toBeInTheDocument();
      expect(screen.getByText(/2 reviews/i)).toBeInTheDocument();
    });
  });

  it('shows 0 tickets and 0 reviews when empty arrays', async () => {
    const prWithEmpty = samplePRs.filter((p) => p.id === 'pr-3');
    mockFetch.mockResolvedValue(jsonResponse({ items: prWithEmpty }));

    renderAtPath('/pull-requests');

    await waitFor(() => {
      expect(screen.getByText(/0 tickets/i)).toBeInTheDocument();
      expect(screen.getByText(/0 reviews/i)).toBeInTheDocument();
    });
  });

  // ─── Filter by status ────────────────────────────────────────────────

  it('passes the status query param to the API', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ items: samplePRs }));

    renderAtPath('/pull-requests?status=open');

    await waitFor(() => {
      expect(screen.getByText('Add user authentication')).toBeInTheDocument();
    });

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining('status=open'),
    );
  });

  it('changes the filter when the status dropdown is used', async () => {
    // First render — return all PRs
    // When status=merged is requested — return only merged PR
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('status=merged')) {
        return jsonResponse({
          items: samplePRs.filter((p) => p.status === 'merged'),
        });
      }
      return jsonResponse({ items: samplePRs });
    });

    const user = userEvent.setup();
    renderAtPath('/pull-requests');

    await waitFor(() => {
      expect(screen.getByText('Add user authentication')).toBeInTheDocument();
    });

    // Change filter to "merged"
    const select = screen.getByRole('combobox', { name: /status/i });
    await user.selectOptions(select, 'merged');

    await waitFor(() => {
      expect(screen.queryByText('Add user authentication')).not.toBeInTheDocument();
      expect(screen.getByText('Fix database migration')).toBeInTheDocument();
    });
  });
});
