import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { PipelineTrigger } from './PipelineTrigger';

// ---- Test wrapper ----

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      mutations: { retry: false },
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

function renderTrigger(runId: string) {
  const wrapper = createWrapper();
  return render(<PipelineTrigger runId={runId} />, { wrapper });
}

// ---- Fixtures ----

function okResponse(): Response {
  return new Response(JSON.stringify({ ok: true }), { status: 202 });
}

function errorResponse(message: string, status = 500): Response {
  return new Response(JSON.stringify({ error: message }), {
    status,
    statusText: message,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('PipelineTrigger', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
    // Provide a token so the Authorization header is attached.
    localStorage.setItem('flux_token', 'test-token');
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  // --- Rendering ---

  it('renders a trigger button with accessible name', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderTrigger('run-1');

    const button = screen.getByRole('button', { name: /trigger/i });
    expect(button).toBeInTheDocument();
    expect(button).toBeEnabled();
  });

  it('renders the button with a descriptive label', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderTrigger('run-abc');

    expect(screen.getByRole('button', { name: /trigger/i })).toBeInTheDocument();
  });

  // --- Click triggers POST ---

  it('POSTs to the correct trigger URL when clicked', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValueOnce(okResponse());

    renderTrigger('run-42');

    await user.click(screen.getByRole('button', { name: /trigger/i }));

    expect(mockFetch).toHaveBeenCalledOnce();
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/v1/pipeline-runs/run-42/trigger',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('sends the Authorization header with the POST request', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValueOnce(okResponse());

    renderTrigger('run-7');

    await user.click(screen.getByRole('button', { name: /trigger/i }));

    const [, options] = mockFetch.mock.lastCall ?? [];
    expect(options).toBeDefined();
    expect(options).toHaveProperty('headers');
    const headers = (options as RequestInit).headers as Record<string, string>;
    expect(headers['Authorization']).toBe('Bearer test-token');
  });

  it('does not send an Authorization header when no token is stored', async () => {
    localStorage.removeItem('flux_token');
    const user = userEvent.setup();
    mockFetch.mockResolvedValueOnce(okResponse());

    renderTrigger('run-noauth');

    await user.click(screen.getByRole('button', { name: /trigger/i }));

    const [, options] = mockFetch.mock.lastCall ?? [];
    const headers = (options as RequestInit).headers as Record<string, string>;
    expect(headers['Authorization']).toBeUndefined();
  });

  // --- Loading state ---

  it('disables the button and shows a spinner while the trigger is in progress', async () => {
    const user = userEvent.setup();
    let resolveTrigger!: (value: Response) => void;
    mockFetch.mockReturnValueOnce(
      new Promise<Response>((r) => {
        resolveTrigger = r;
      }),
    );

    renderTrigger('run-pending');

    const button = screen.getByRole('button', { name: /trigger/i });
    await user.click(button);

    expect(button).toBeDisabled();
    expect(screen.getByLabelText(/triggering/i)).toBeInTheDocument();

    // Finalize the request and verify spinner is removed.
    resolveTrigger(okResponse());

    await waitFor(() => {
      expect(screen.queryByLabelText(/triggering/i)).not.toBeInTheDocument();
    });
  });

  // --- Error state ---

  it('shows an error message when the trigger request fails', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValueOnce(errorResponse('Orchestrator unavailable', 503));

    renderTrigger('run-fail');

    await user.click(screen.getByRole('button', { name: /trigger/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/orchestrator unavailable/i)).toBeInTheDocument();
    });
  });

  it('re-enables the button after a failed trigger', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValueOnce(errorResponse('Conflict', 409));

    renderTrigger('run-conflict');

    const button = screen.getByRole('button', { name: /trigger/i });
    await user.click(button);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    expect(button).toBeEnabled();
  });

  // --- Success state ---

  it('shows a success message after a successful trigger', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValueOnce(okResponse());

    renderTrigger('run-ok');

    await user.click(screen.getByRole('button', { name: /trigger/i }));

    await waitFor(() => {
      expect(screen.getByText(/triggered/i)).toBeInTheDocument();
    });
  });

  it('re-enables the button after a successful trigger', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValueOnce(okResponse());

    renderTrigger('run-ok');

    const button = screen.getByRole('button', { name: /trigger/i });
    await user.click(button);

    await waitFor(() => {
      expect(screen.getByText(/triggered/i)).toBeInTheDocument();
    });

    expect(button).toBeEnabled();
  });
});
