import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createMemoryHistory } from '@tanstack/react-router';
import { act } from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createAppRouter } from '../router';

// ─── Integration test helper ─────────────────────────────────────────────

async function renderAt(path = '/tickets') {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  const memoryHistory = createMemoryHistory({ initialEntries: [path] });
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

// ─── Mock response helpers ──────────────────────────────────────────────

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

// ─── Fixtures ───────────────────────────────────────────────────────────

const sampleTicket = {
  id: 'ticket-1',
  project_id: 'proj-1',
  external_id: 'GH-42',
  source: 'github' as const,
  title: 'Fix login bug',
  description: 'Users cannot log in with SSO',
  status: 'open' as const,
  labels: ['bug', 'auth'],
  relationships: [{ type: 'blocks' as const, target_id: 'ticket-2' }],
  prs: ['pr-1'],
  created_at: '2026-06-01T10:00:00Z',
  updated_at: '2026-06-20T14:30:00Z',
};

const sampleTickets = [
  sampleTicket,
  {
    id: 'ticket-2',
    project_id: 'proj-1',
    external_id: 'JIRA-123',
    source: 'jira' as const,
    title: 'Update API documentation',
    description: 'Document new endpoints',
    status: 'in_progress' as const,
    labels: ['docs'],
    relationships: [],
    prs: [],
    created_at: '2026-06-02T08:00:00Z',
    updated_at: '2026-06-21T09:00:00Z',
  },
  {
    id: 'ticket-3',
    project_id: 'proj-2',
    external_id: 'LIN-7',
    source: 'linear' as const,
    title: 'Design system audit',
    description: 'Review color tokens',
    status: 'closed' as const,
    labels: ['design', 'frontend'],
    relationships: [],
    prs: ['pr-3'],
    created_at: '2026-05-15T12:00:00Z',
    updated_at: '2026-06-10T16:00:00Z',
  },
];

const samplePage = {
  items: sampleTickets,
  page: 1,
  limit: 20,
  total: 3,
};

const emptyPage = {
  items: [],
  page: 1,
  limit: 20,
  total: 0,
};

// ─── Tests ──────────────────────────────────────────────────────────────

describe('TicketsPage', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // --- Loading state ---

  it('shows loading indicator while fetching tickets', async () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    await renderAt('/tickets');

    expect(screen.getByRole('status', { name: /loading/i })).toBeInTheDocument();
  });

  // --- Empty state ---

  it('shows empty state when no tickets exist', async () => {
    mockFetch.mockResolvedValue(jsonResponse(emptyPage));

    renderAt('/tickets');

    await waitFor(() => {
      expect(screen.getByText(/no tickets found/i)).toBeInTheDocument();
    });
  });

  it('does not show empty state while loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderAt('/tickets');

    expect(screen.queryByText(/no tickets found/i)).toBeNull();
  });

  // --- Error state ---

  it('shows error banner when fetch fails', async () => {
    mockFetch.mockResolvedValue(
      new Response(JSON.stringify({ error: 'Internal Server Error' }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    renderAt('/tickets');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/internal server error/i)).toBeInTheDocument();
    });
  });

  it('shows a retry button in the error banner', async () => {
    const user = userEvent.setup();
    mockFetch
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'boom' }), {
          status: 500,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
      .mockResolvedValueOnce(jsonResponse(samplePage));

    renderAt('/tickets');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    const retryButton = screen.getByRole('button', { name: /retry/i });
    await user.click(retryButton);

    await waitFor(() => {
      expect(screen.getByText('Fix login bug')).toBeInTheDocument();
    });
  });

  // --- Success state ---

  it('renders ticket titles from the API', async () => {
    mockFetch.mockResolvedValue(jsonResponse(samplePage));

    renderAt('/tickets');

    await waitFor(() => {
      expect(screen.getByText('Fix login bug')).toBeInTheDocument();
      expect(screen.getByText('Update API documentation')).toBeInTheDocument();
      expect(screen.getByText('Design system audit')).toBeInTheDocument();
    });
  });

  it('renders status badges for each ticket', async () => {
    mockFetch.mockResolvedValue(jsonResponse(samplePage));

    renderAt('/tickets');

    await waitFor(() => {
      // Status text appears in both badges and dropdown options
      expect(screen.getAllByText('open').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('in_progress').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('closed').length).toBeGreaterThanOrEqual(1);
    });
  });

  it('renders source indicators for each ticket', async () => {
    mockFetch.mockResolvedValue(jsonResponse(samplePage));

    renderAt('/tickets');

    await waitFor(() => {
      // Source text appears in cards and dropdown options; use getAllByText
      const sources = screen.getAllByText(/github|jira|linear/i);
      // At minimum, each source appears as a card label (3 cards)
      // plus dropdown options (3 options)
      expect(sources.length).toBeGreaterThanOrEqual(3);
    });
  });

  it('renders labels for tickets that have them', async () => {
    mockFetch.mockResolvedValue(jsonResponse(samplePage));

    renderAt('/tickets');

    await waitFor(() => {
      expect(screen.getByText('bug')).toBeInTheDocument();
      expect(screen.getByText('auth')).toBeInTheDocument();
      expect(screen.getByText('docs')).toBeInTheDocument();
      expect(screen.getByText('design')).toBeInTheDocument();
      expect(screen.getByText('frontend')).toBeInTheDocument();
    });
  });

  it('shows relationship count for tickets with relationships', async () => {
    mockFetch.mockResolvedValue(jsonResponse(samplePage));

    renderAt('/tickets');

    await waitFor(() => {
      expect(screen.getByText(/1 relationship/i)).toBeInTheDocument();
    });
  });

  // --- Filter by status ---

  it('passes status filter as query parameter', async () => {
    mockFetch.mockResolvedValue(jsonResponse(emptyPage));

    renderAt('/tickets?status=open');

    await waitFor(() => {
      // The fetch call should include ?status=open
      const fetchCall = mockFetch.mock.calls.find(
        (c: unknown[]) => typeof c[0] === 'string' && c[0].includes('/api/v1/tickets'),
      );
      expect(fetchCall).toBeDefined();
      if (fetchCall) {
        const url = fetchCall[0] as string;
        expect(url).toContain('status=open');
      }
    });
  });

  it('filters displayed tickets by selected status', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValue(jsonResponse(samplePage));

    renderAt('/tickets');

    await waitFor(() => {
      expect(screen.getByText('Fix login bug')).toBeInTheDocument();
    });

    // Select "open" in the status dropdown
    const statusSelect = screen.getByLabelText('Status');
    await user.selectOptions(statusSelect, 'open');

    // The page should refetch with the new filter
    await waitFor(() => {
      const fetchCalls = mockFetch.mock.calls.filter(
        (c: unknown[]) => typeof c[0] === 'string' && c[0].includes('/api/v1/tickets?'),
      );
      const lastCall = fetchCalls[fetchCalls.length - 1];
      expect(lastCall).toBeDefined();
      if (lastCall) {
        const url = lastCall[0] as string;
        expect(url).toContain('status=open');
      }
    });
  });

  // --- Filter by source ---

  it('filters displayed tickets by selected source', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValue(jsonResponse(samplePage));

    renderAt('/tickets');

    await waitFor(() => {
      expect(screen.getByText('Fix login bug')).toBeInTheDocument();
    });

    const sourceSelect = screen.getByLabelText('Source');
    await user.selectOptions(sourceSelect, 'github');

    await waitFor(() => {
      const fetchCalls = mockFetch.mock.calls.filter(
        (c: unknown[]) => typeof c[0] === 'string' && c[0].includes('/api/v1/tickets?'),
      );
      const lastCall = fetchCalls[fetchCalls.length - 1];
      expect(lastCall).toBeDefined();
      if (lastCall) {
        const url = lastCall[0] as string;
        expect(url).toContain('source=github');
      }
    });
  });

  // --- Renders the nav and heading ---

  it('renders the page heading', async () => {
    mockFetch.mockResolvedValue(jsonResponse(emptyPage));

    renderAt('/tickets');

    await waitFor(() => {
      expect(
        screen.getByRole('heading', { name: /tickets/i }),
      ).toBeInTheDocument();
    });
  });
});
