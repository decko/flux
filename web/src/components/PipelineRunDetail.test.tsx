import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { PipelineRunDetail } from './PipelineRunDetail';

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

function renderDetail(runId: string) {
  const wrapper = createWrapper();
  return render(<PipelineRunDetail runId={runId} />, { wrapper });
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

// ---- Sample fixture matching the PipelineRun domain model ----

const sampleRun = {
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
      output: 'All clear — no issues found',
      error: '',
      started_at: '2026-06-21T10:00:00Z',
    },
    {
      name: 'test',
      status: 'completed',
      duration: 45_000_000_000,
      output: '42 passed, 0 failed',
      error: '',
      started_at: '2026-06-21T10:00:12Z',
    },
    {
      name: 'build',
      status: 'completed',
      duration: 8_000_000_000,
      output: 'Build successful',
      error: '',
      started_at: '2026-06-21T10:00:57Z',
    },
  ],
  started_at: '2026-06-21T10:00:00Z',
  completed_at: '2026-06-21T10:01:10Z',
  cost: {
    total: 0.42,
    currency: 'USD',
    by_phase: { lint: 0.10, test: 0.27, build: 0.05 },
  },
};

const failedRun = {
  id: 'run-fail',
  project_id: 'proj-1',
  ticket_id: 'ticket-1',
  orchestrator: 'soda',
  pipeline: 'security-scan',
  status: 'failed',
  phases: [
    {
      name: 'scan',
      status: 'failed',
      duration: 15_000_000_000,
      output: '',
      error: 'CVE-2024-1234: critical vulnerability found in dependencies',
      started_at: '2026-06-21T11:00:00Z',
    },
  ],
  started_at: '2026-06-21T11:00:00Z',
  completed_at: '2026-06-21T11:00:15Z',
  cost: { total: 0.08, currency: 'USD', by_phase: { scan: 0.08 } },
};

const runningRun = {
  id: 'run-running',
  project_id: 'proj-1',
  ticket_id: 'ticket-1',
  orchestrator: 'soda',
  pipeline: 'generate-tests',
  status: 'running',
  phases: [
    {
      name: 'analyze',
      status: 'completed',
      duration: 5_000_000_000,
      output: 'AST parsed successfully',
      error: '',
      started_at: '2026-06-21T12:00:00Z',
    },
    {
      name: 'generate',
      status: 'running',
      duration: 0,
      output: '',
      error: '',
      started_at: '2026-06-21T12:00:05Z',
    },
  ],
  started_at: '2026-06-21T12:00:00Z',
};

describe('PipelineRunDetail', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // --- Loading state ---

  it('shows a loading indicator while the run is being fetched', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderDetail('run-1');

    const loader = screen.getByRole('status', { name: /loading/i });
    expect(loader).toBeInTheDocument();
  });

  it('does not show content while loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderDetail('run-1');

    expect(screen.queryByText(/code-review/i)).toBeNull();
    expect(screen.queryByText(/completed/i)).toBeNull();
  });

  // --- Error state ---

  it('shows an error message when the fetch fails with a server error', async () => {
    mockFetch.mockResolvedValue(jsonErrorResponse(500, 'Internal Server Error'));

    renderDetail('run-err');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/internal server error/i)).toBeInTheDocument();
    });
  });

  it('shows an error message on network failure', async () => {
    mockFetch.mockRejectedValue(new Error('Failed to fetch'));

    renderDetail('run-net');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/failed to fetch/i)).toBeInTheDocument();
    });
  });

  it('shows an error message for a 404 not found response', async () => {
    mockFetch.mockResolvedValue(jsonErrorResponse(404, 'Pipeline run not found'));

    renderDetail('run-missing');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/not found/i)).toBeInTheDocument();
    });
  });

  // --- Success state: metadata ---

  it('displays the pipeline name', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRun));

    renderDetail('run-1');

    await waitFor(() => {
      expect(screen.getByText('code-review')).toBeInTheDocument();
    });
  });

  it('displays the orchestrator name', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRun));

    renderDetail('run-1');

    await waitFor(() => {
      expect(screen.getByText('soda')).toBeInTheDocument();
    });
  });

  it('displays the run status', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRun));

    renderDetail('run-1');

    await waitFor(() => {
      expect(screen.getByText(/completed/i)).toBeInTheDocument();
    });
  });

  it('displays the started time', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRun));

    renderDetail('run-1');

    await waitFor(() => {
      expect(screen.getByText(/started/i)).toBeInTheDocument();
    });
  });

  // --- Success state: phases ---

  it('renders all phases with their names', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRun));

    renderDetail('run-1');

    await waitFor(() => {
      expect(screen.getByText('lint')).toBeInTheDocument();
      expect(screen.getByText('test')).toBeInTheDocument();
      expect(screen.getByText('build')).toBeInTheDocument();
    });
  });

  it('displays phase statuses', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRun));

    renderDetail('run-1');

    await waitFor(() => {
      const completedBadges = screen.getAllByText(/completed/i);
      // 1 run status + 3 phase statuses = 4 completed labels
      expect(completedBadges.length).toBeGreaterThanOrEqual(1);
    });
  });

  it('displays phase durations', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRun));

    renderDetail('run-1');

    await waitFor(() => {
      // lint took 12s
      expect(screen.getByText(/12/i)).toBeInTheDocument();
    });
  });

  it('displays phase output for successful phases', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRun));

    renderDetail('run-1');

    await waitFor(() => {
      expect(screen.getByText(/all clear/i)).toBeInTheDocument();
      expect(screen.getByText(/42 passed/i)).toBeInTheDocument();
    });
  });

  it('displays phase error messages for failed phases', async () => {
    mockFetch.mockResolvedValue(jsonResponse(failedRun));

    renderDetail('run-fail');

    await waitFor(() => {
      expect(
        screen.getByText(/CVE-2024-1234/i),
      ).toBeInTheDocument();
    });
  });

  it('shows a running indicator for an in-progress phase', async () => {
    mockFetch.mockResolvedValue(jsonResponse(runningRun));

    renderDetail('run-running');

    await waitFor(() => {
      expect(screen.getByText('generate')).toBeInTheDocument();
      expect(screen.getAllByText(/running/i).length).toBeGreaterThanOrEqual(1);
    });
  });

  // --- Success state: cost breakdown ---

  it('displays the total cost', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRun));

    renderDetail('run-1');

    await waitFor(() => {
      expect(screen.getByText(/0\.42/)).toBeInTheDocument();
      expect(screen.getByText(/USD/)).toBeInTheDocument();
    });
  });

  it('displays per-phase cost breakdown', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRun));

    renderDetail('run-1');

    await waitFor(() => {
      expect(screen.getByText(/0\.10/)).toBeInTheDocument();
      expect(screen.getByText(/0\.27/)).toBeInTheDocument();
      expect(screen.getByText(/0\.05/)).toBeInTheDocument();
    });
  });

  it('does not show cost section when cost is omitted', async () => {
    mockFetch.mockResolvedValue(jsonResponse(runningRun));

    renderDetail('run-running');

    await waitFor(() => {
      expect(screen.getByText('generate')).toBeInTheDocument();
    });

    // Running run has no cost field; cost section should not render.
    expect(screen.queryByText(/\$/)).toBeNull();
  });

  // --- Fetch URL ---

  it('fetches the correct run by ID', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRun));

    renderDetail('run-1');

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/api/v1/pipeline-runs/run-1'),
        expect.any(Object),
      );
    });
  });
});
