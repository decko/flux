import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { InstallationPicker } from './InstallationPicker';
import type { GitHubInstallation } from '@/api/github';

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

function renderPicker(onSelect?: (installation: GitHubInstallation) => void) {
  const wrapper = createWrapper();
  return render(
    <InstallationPicker onSelect={onSelect ?? vi.fn()} />,
    { wrapper },
  );
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

// ---- Sample fixtures ----

const sampleInstallations: GitHubInstallation[] = [
  {
    id: 101,
    account: { login: 'flux-org' },
    target_type: 'Organization',
    html_url: 'https://github.com/organizations/flux-org',
  },
  {
    id: 202,
    account: { login: 'decko' },
    target_type: 'User',
    html_url: 'https://github.com/decko',
  },
];

describe('InstallationPicker', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
    localStorage.setItem('flux_token', 'test-token');
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  // --- Loading state ---

  it('renders loading skeleton while fetching', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPicker();

    const skeleton = screen.getByRole('status', { name: /loading/i });
    expect(skeleton).toBeInTheDocument();
  });

  it('does not show empty state while loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPicker();

    expect(screen.queryByText(/no installations/i)).toBeNull();
  });

  // --- Success state ---

  it('renders installation cards on success', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleInstallations));

    renderPicker();

    await waitFor(() => {
      expect(screen.getByText('flux-org')).toBeInTheDocument();
      expect(screen.getByText('decko')).toBeInTheDocument();
    });
  });

  it('calls onSelect when card is clicked', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    mockFetch.mockResolvedValue(jsonResponse(sampleInstallations));

    renderPicker(onSelect);

    await waitFor(() => {
      expect(screen.getByText('flux-org')).toBeInTheDocument();
    });

    await user.click(screen.getByText('flux-org'));

    expect(onSelect).toHaveBeenCalledOnce();
    expect(onSelect).toHaveBeenCalledWith(sampleInstallations[0]);
  });

  // --- Error state ---

  it('renders error message on fetch failure', async () => {
    mockFetch.mockResolvedValue(
      jsonErrorResponse(500, 'Internal Server Error'),
    );

    renderPicker();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/internal server error/i)).toBeInTheDocument();
    });
  });

  // --- Empty state ---

  it('renders empty state when no installations', async () => {
    mockFetch.mockResolvedValue(jsonResponse([]));

    renderPicker();

    await waitFor(() => {
      expect(screen.getByText(/no installations found/i)).toBeInTheDocument();
    });
  });

  // --- 503 state ---

  it('renders GitHub App not configured message on 503', async () => {
    mockFetch.mockResolvedValue(
      jsonErrorResponse(503, 'GitHub App not configured'),
    );

    renderPicker();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(
        screen.getByText(/github app not configured/i),
      ).toBeInTheDocument();
    });
  });
});
