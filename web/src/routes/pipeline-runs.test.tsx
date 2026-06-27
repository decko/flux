import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { PipelineRunsPage } from './pipeline-runs';

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
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

function renderPage() {
  const wrapper = createWrapper();
  return render(<PipelineRunsPage />, { wrapper });
}

// ---- Helpers ----

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

// ---- Sample fixtures matching the PipelineRun domain model ----

const sampleRuns = [
  {
    id: 'run-1',
    project_id: 'proj-1',
    ticket_id: 'ticket-1',
    orchestrator: 'soda',
    pipeline: 'code-review',
    status: 'completed',
    phases: [
      {
        name: 'lint',
        status: 'completed',
        duration: 12_000_000_000,
        output: 'All clear',
        error: '',
        started_at: '2026-06-21T10:00:00Z',
      },
      {
        name: 'test',
        status: 'completed',
        duration: 45_000_000_000,
        output: '42 passed',
        error: '',
        started_at: '2026-06-21T10:00:12Z',
      },
    ],
    started_at: '2026-06-21T10:00:00Z',
    completed_at: '2026-06-21T10:01:00Z',
  },
  {
    id: 'run-2',
    project_id: 'proj-1',
    ticket_id: 'ticket-2',
    orchestrator: 'soda',
    pipeline: 'security-scan',
    status: 'failed',
    phases: [
      {
        name: 'scan',
        status: 'failed',
        duration: 30_000_000_000,
        output: '',
        error: 'CVE-2024-1234 found',
        started_at: '2026-06-21T11:00:00Z',
      },
    ],
    started_at: '2026-06-21T11:00:00Z',
    completed_at: '2026-06-21T11:00:30Z',
  },
];

describe('PipelineRunsPage', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // --- Page structure ---

  it('renders the page heading and description', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPage();

    expect(screen.getByText('Pipeline Runs')).toBeInTheDocument();
    expect(
      screen.getByText(/monitor pipeline execution status/i),
    ).toBeInTheDocument();
  });

  it('renders a ticket_id filter input with an apply button', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPage();

    expect(
      screen.getByPlaceholderText(/filter by ticket/i),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /apply/i }),
    ).toBeInTheDocument();
  });

  // --- Loading state ---

  it('shows loading skeletons while pipeline runs are being fetched', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPage();

    const skeletons = screen.getByRole('status', { name: /loading/i });
    expect(skeletons).toBeInTheDocument();
  });

  it('does not show empty state while still loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPage();

    expect(screen.queryByText(/no pipeline runs/i)).toBeNull();
  });

  it('does not show error state while still loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPage();

    expect(screen.queryByRole('alert')).toBeNull();
  });

  // --- Empty state ---

  it('shows an empty state message when there are no pipeline runs', async () => {
    mockFetch.mockResolvedValue(jsonResponse([]));

    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/no pipeline runs/i)).toBeInTheDocument();
    });
  });

  // --- Error state ---

  it('shows an error banner when the fetch fails', async () => {
    mockFetch.mockResolvedValue(jsonErrorResponse(500, 'Internal Server Error'));

    renderPage();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/internal server error/i)).toBeInTheDocument();
    });
  });

  it('shows an error banner on network failure', async () => {
    mockFetch.mockRejectedValue(new Error('Network Error'));

    renderPage();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/network error/i)).toBeInTheDocument();
    });
  });

  // --- Success state: rendering runs ---

  it('renders pipeline run cards when data is loaded', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ items: sampleRuns }));

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('code-review')).toBeInTheDocument();
      expect(screen.getByText('security-scan')).toBeInTheDocument();
    });
  });

  it('renders the correct number of run cards', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ items: sampleRuns }));

    renderPage();

    await waitFor(() => {
      const cards = screen.getAllByTestId('pipeline-run-card');
      expect(cards).toHaveLength(sampleRuns.length);
    });
  });

  it('displays status badges for each run', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ items: sampleRuns }));

    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/completed/i)).toBeInTheDocument();
      expect(screen.getByText(/failed/i)).toBeInTheDocument();
    });
  });

  // --- Filter interaction ---

  it('filters by ticket_id when the filter is applied', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValue(jsonResponse([]));

    renderPage();

    const input = screen.getByPlaceholderText(/filter by ticket/i);
    await user.type(input, 'ticket-42');

    const applyButton = screen.getByRole('button', { name: /apply/i });
    await user.click(applyButton);

    // PipelineRunList should now fetch with the ticket_id query param
    await waitFor(() => {
      const lastCall = mockFetch.mock.lastCall;
      expect(lastCall).toBeDefined();
      const url = lastCall?.[0] as string;
      expect(url).toContain('ticket_id=ticket-42');
    });
  });

  it('applies filter on Enter key press in the input', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValue(jsonResponse([]));

    renderPage();

    const input = screen.getByPlaceholderText(/filter by ticket/i);
    await user.type(input, 'ticket-99{Enter}');

    await waitFor(() => {
      const lastCall = mockFetch.mock.lastCall;
      expect(lastCall).toBeDefined();
      const url = lastCall?.[0] as string;
      expect(url).toContain('ticket_id=ticket-99');
    });
  });

  it('clears filter when input is empty and apply is clicked', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValue(jsonResponse([]));

    renderPage();

    // First apply a filter
    const input = screen.getByPlaceholderText(/filter by ticket/i);
    await user.type(input, 'ticket-42');
    const applyButton = screen.getByRole('button', { name: /apply/i });
    await user.click(applyButton);

    await waitFor(() => {
      const lastCall = mockFetch.mock.lastCall;
      expect(lastCall).toBeDefined();
      const url = lastCall?.[0] as string;
      expect(url).toContain('ticket_id=ticket-42');
    });

    // Clear the input and apply again
    await user.clear(input);
    await user.click(applyButton);

    // Should now fetch without ticket_id param (all runs)
    await waitFor(() => {
      const lastCall = mockFetch.mock.lastCall;
      expect(lastCall).toBeDefined();
      const url = lastCall?.[0] as string;
      expect(url).toBe('/api/v1/pipeline-runs');
    });
  });
});
