import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { PipelineRunList } from './PipelineRunList';

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

function renderList(ticketId: string) {
  const wrapper = createWrapper();
  return render(<PipelineRunList ticketId={ticketId} />, { wrapper });
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
      { name: 'lint', status: 'completed', duration: 12_000_000_000, output: 'All clear', error: '', started_at: '2026-06-21T10:00:00Z' },
      { name: 'test', status: 'completed', duration: 45_000_000_000, output: '42 passed', error: '', started_at: '2026-06-21T10:00:12Z' },
    ],
    started_at: '2026-06-21T10:00:00Z',
    completed_at: '2026-06-21T10:01:00Z',
    cost: { total: 0.42, currency: 'USD', by_phase: { lint: 0.10, test: 0.32 } },
  },
  {
    id: 'run-2',
    project_id: 'proj-1',
    ticket_id: 'ticket-1',
    orchestrator: 'soda',
    pipeline: 'security-scan',
    status: 'failed',
    phases: [
      { name: 'scan', status: 'failed', duration: 30_000_000_000, output: '', error: 'CVE-2024-1234 found', started_at: '2026-06-21T11:00:00Z' },
    ],
    started_at: '2026-06-21T11:00:00Z',
    completed_at: '2026-06-21T11:00:30Z',
    cost: { total: 0.15, currency: 'USD', by_phase: { scan: 0.15 } },
  },
  {
    id: 'run-3',
    project_id: 'proj-1',
    ticket_id: 'ticket-1',
    orchestrator: 'soda',
    pipeline: 'generate-tests',
    status: 'running',
    phases: [
      { name: 'analyze', status: 'completed', duration: 5_000_000_000, output: 'AST parsed', error: '', started_at: '2026-06-21T12:00:00Z' },
      { name: 'generate', status: 'running', duration: 0, output: '', error: '', started_at: '2026-06-21T12:00:05Z' },
    ],
    started_at: '2026-06-21T12:00:00Z',
  },
];

describe('PipelineRunList', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // --- Loading state ---

  it('shows loading skeletons while pipeline runs are being fetched', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderList('ticket-1');

    const skeletons = screen.getByRole('status', { name: /loading/i });
    expect(skeletons).toBeInTheDocument();
  });

  it('does not show empty state while still loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderList('ticket-1');

    expect(screen.queryByText(/no pipeline runs/i)).toBeNull();
  });

  it('does not show error state while still loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderList('ticket-1');

    expect(screen.queryByRole('alert')).toBeNull();
  });

  // --- Empty state ---

  it('shows an empty state message when there are no pipeline runs', async () => {
    mockFetch.mockResolvedValue(jsonResponse([]));

    renderList('ticket-empty');

    await waitFor(() => {
      expect(screen.getByText(/no pipeline runs/i)).toBeInTheDocument();
    });
  });

  // --- Error state ---

  it('shows an error banner when the fetch fails', async () => {
    mockFetch.mockResolvedValue(jsonErrorResponse(500, 'Internal Server Error'));

    renderList('ticket-err');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/internal server error/i)).toBeInTheDocument();
    });
  });

  it('shows an error banner on network failure', async () => {
    mockFetch.mockRejectedValue(new Error('Network Error'));

    renderList('ticket-net');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/network error/i)).toBeInTheDocument();
    });
  });

  // --- Success state: rendering runs ---

  it('renders pipeline run cards when data is loaded', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRuns));

    renderList('ticket-1');

    await waitFor(() => {
      expect(screen.getByText('code-review')).toBeInTheDocument();
      expect(screen.getByText('security-scan')).toBeInTheDocument();
      expect(screen.getByText('generate-tests')).toBeInTheDocument();
    });
  });

  it('renders the correct number of run cards', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRuns));

    renderList('ticket-1');

    await waitFor(() => {
      const cards = screen.getAllByTestId('pipeline-run-card');
      expect(cards).toHaveLength(sampleRuns.length);
    });
  });

  it('displays the orchestrator name for each run', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRuns));

    renderList('ticket-1');

    await waitFor(() => {
      const sodaLabels = screen.getAllByText('soda');
      expect(sodaLabels.length).toBeGreaterThanOrEqual(1);
    });
  });

  // --- Status badges ---

  it('displays status badges for each run', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRuns));

    renderList('ticket-1');

    await waitFor(() => {
      expect(screen.getByText(/completed/i)).toBeInTheDocument();
      expect(screen.getByText(/failed/i)).toBeInTheDocument();
      expect(screen.getByText(/running/i)).toBeInTheDocument();
    });
  });

  it('renders status badges with semantic labels', async () => {
    mockFetch.mockResolvedValue(jsonResponse([sampleRuns[0]]));

    renderList('ticket-1');

    await waitFor(() => {
      const badge = screen.getByLabelText(/status: completed/i);
      expect(badge).toBeInTheDocument();
    });
  });

  // --- Phase count ---

  it('displays the phase count for each run', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRuns));

    renderList('ticket-1');

    await waitFor(() => {
      // run-1 has 2 phases, run-2 has 1 phase, run-3 has 2 phases
      const twoPhaseElements = screen.getAllByText(/2 phases/i);
      expect(twoPhaseElements.length).toBeGreaterThanOrEqual(1);
      expect(screen.getByText(/1 phase/i)).toBeInTheDocument();
    });
  });

  // --- Started time ---

  it('displays the started time for each run', async () => {
    mockFetch.mockResolvedValue(jsonResponse([sampleRuns[0]]));

    renderList('ticket-1');

    await waitFor(() => {
      expect(screen.getByText(/started/i)).toBeInTheDocument();
    });
  });

  // --- Fetch URL ---

  it('fetches pipeline runs filtered by ticket_id', async () => {
    mockFetch.mockResolvedValue(jsonResponse([]));

    renderList('ticket-42');

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/api/v1/pipeline-runs'),
        expect.any(Object),
      );
    });

    const url = mockFetch.mock.calls[0]?.[0] as string;
    expect(url).toContain('ticket_id=ticket-42');
  });
});
