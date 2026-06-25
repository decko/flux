import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { RepositoryPicker } from './RepositoryPicker';
import type { GitHubInstallationRepo } from '@/api/github';

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

function renderPicker(
  installationId: string,
  onSelect?: (repo: GitHubInstallationRepo) => void,
) {
  const wrapper = createWrapper();
  return render(
    <RepositoryPicker
      installationId={installationId}
      onSelect={onSelect ?? vi.fn()}
    />,
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

const sampleRepos: GitHubInstallationRepo[] = [
  {
    id: 1,
    name: 'flux',
    full_name: 'flux-org/flux',
    html_url: 'https://github.com/flux-org/flux',
    private: false,
  },
  {
    id: 2,
    name: 'web-app',
    full_name: 'flux-org/web-app',
    html_url: 'https://github.com/flux-org/web-app',
    private: true,
  },
];

describe('RepositoryPicker', () => {
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

  it('renders loading state while fetching', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPicker('101');

    const skeleton = screen.getByRole('status', { name: /loading/i });
    expect(skeleton).toBeInTheDocument();
  });

  it('does not show empty state while loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPicker('101');

    expect(screen.queryByText(/no repositories found/i)).toBeNull();
  });

  // --- Success state ---

  it('renders repository list on success', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRepos));

    renderPicker('101');

    await waitFor(() => {
      expect(screen.getByText('flux')).toBeInTheDocument();
      expect(screen.getByText('flux-org/flux')).toBeInTheDocument();
      expect(screen.getByText('web-app')).toBeInTheDocument();
      expect(screen.getByText('flux-org/web-app')).toBeInTheDocument();
    });
  });

  it('calls onSelect when repo is clicked', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    mockFetch.mockResolvedValue(jsonResponse(sampleRepos));

    renderPicker('101', onSelect);

    await waitFor(() => {
      expect(screen.getByText('flux')).toBeInTheDocument();
    });

    await user.click(screen.getByText('flux'));

    expect(onSelect).toHaveBeenCalledOnce();
    expect(onSelect).toHaveBeenCalledWith(sampleRepos[0]);
  });

  // --- Error state ---

  it('renders error on fetch failure', async () => {
    mockFetch.mockResolvedValue(
      jsonErrorResponse(500, 'Internal Server Error'),
    );

    renderPicker('101');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/internal server error/i)).toBeInTheDocument();
    });
  });

  // --- Empty state ---

  it('renders empty state when no repos', async () => {
    mockFetch.mockResolvedValue(jsonResponse([]));

    renderPicker('101');

    await waitFor(() => {
      expect(
        screen.getByText(/no repositories found/i),
      ).toBeInTheDocument();
    });
  });

  // --- 503 state ---

  it('renders GitHub App not configured message on 503', async () => {
    mockFetch.mockResolvedValue(
      jsonErrorResponse(503, 'GitHub App not configured'),
    );

    renderPicker('101');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(
        screen.getByText(/github app not configured/i),
      ).toBeInTheDocument();
    });
  });
});
