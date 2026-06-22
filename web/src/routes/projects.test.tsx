import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ProjectsPage } from './projects';

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

function renderPage() {
  const wrapper = createWrapper();
  return render(<ProjectsPage />, { wrapper });
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

// ---- Sample fixtures matching the Project model ----

const sampleProjects = [
  {
    id: 'proj-1',
    name: 'flux-core',
    repo_url: 'https://github.com/decko/flux',
    definition: {
      language: 'Go',
      framework: '',
      conventions: ['conventional-commits'],
      architecture: 'hexagonal',
    },
    adapters: [],
    pipelines: [],
    created_at: '2026-06-01T00:00:00Z',
    updated_at: '2026-06-01T00:00:00Z',
  },
  {
    id: 'proj-2',
    name: 'web-app',
    repo_url: 'https://github.com/decko/web-app',
    definition: {
      language: 'TypeScript',
      framework: 'React',
      conventions: ['prettier', 'eslint'],
      architecture: 'SPA',
    },
    adapters: [],
    pipelines: [],
    created_at: '2026-06-02T00:00:00Z',
    updated_at: '2026-06-02T00:00:00Z',
  },
];

describe('ProjectsPage', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // --- Loading state ---

  it('shows loading skeletons while projects are being fetched', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPage();

    const skeletons = screen.getByRole('status', { name: /loading/i });
    expect(skeletons).toBeInTheDocument();
  });

  it('does not show empty state while still loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPage();

    expect(screen.queryByText(/no projects/i)).toBeNull();
  });

  it('does not show error state while still loading', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPage();

    expect(screen.queryByRole('alert')).toBeNull();
  });

  // --- Empty state ---

  it('shows an empty state message when there are no projects', async () => {
    mockFetch.mockResolvedValue(jsonResponse([]));

    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/no projects/i)).toBeInTheDocument();
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

  // --- Success state: rendering projects ---

  it('renders project names when data is loaded', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleProjects));

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('flux-core')).toBeInTheDocument();
      expect(screen.getByText('web-app')).toBeInTheDocument();
    });
  });

  it('renders the correct number of project cards', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleProjects));

    renderPage();

    await waitFor(() => {
      const cards = screen.getAllByTestId('project-card');
      expect(cards).toHaveLength(sampleProjects.length);
    });
  });

  it('displays the repo URL for each project', async () => {
    mockFetch.mockResolvedValue(jsonResponse([sampleProjects[0]]));

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('https://github.com/decko/flux')).toBeInTheDocument();
    });
  });

  // --- Create form ---

  it('renders the create project form', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPage();

    expect(screen.getByLabelText(/project name/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/repo url/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create/i })).toBeInTheDocument();
  });

  it('sends a POST request with name and repo_url when creating a project', async () => {
    const user = userEvent.setup();

    mockFetch.mockResolvedValueOnce(jsonResponse([]));

    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/no projects/i)).toBeInTheDocument();
    });

    const newProject = {
      id: 'proj-3',
      name: 'new-project',
      repo_url: 'https://github.com/example/new',
      definition: { language: '', framework: '', conventions: [], architecture: '' },
      adapters: [],
      pipelines: [],
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    };

    mockFetch.mockResolvedValueOnce(jsonResponse(newProject, 201));
    mockFetch.mockResolvedValueOnce(jsonResponse([newProject]));

    await user.type(screen.getByLabelText(/project name/i), 'new-project');
    await user.type(screen.getByLabelText(/repo url/i), 'https://github.com/example/new');
    await user.click(screen.getByRole('button', { name: /create/i }));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/projects',
        expect.objectContaining({ method: 'POST' }),
      );
    });

    const postCall = mockFetch.mock.calls.find(
      ([url, opts]) => url === '/api/v1/projects' && (opts as RequestInit).method === 'POST',
    )!;
    const body = JSON.parse((postCall[1] as RequestInit).body as string);
    expect(body).toEqual({
      name: 'new-project',
      repo_url: 'https://github.com/example/new',
    });
  });

  it('shows the newly created project in the list', async () => {
    const user = userEvent.setup();

    mockFetch.mockResolvedValueOnce(jsonResponse([]));

    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/no projects/i)).toBeInTheDocument();
    });

    const newProject = {
      id: 'proj-3',
      name: 'new-project',
      repo_url: 'https://github.com/example/new',
      definition: { language: '', framework: '', conventions: [], architecture: '' },
      adapters: [],
      pipelines: [],
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    };

    mockFetch.mockResolvedValueOnce(jsonResponse(newProject, 201));
    mockFetch.mockResolvedValueOnce(jsonResponse([newProject]));

    await user.type(screen.getByLabelText(/project name/i), 'new-project');
    await user.type(screen.getByLabelText(/repo url/i), 'https://github.com/example/new');
    await user.click(screen.getByRole('button', { name: /create/i }));

    await waitFor(() => {
      expect(screen.getByText('new-project')).toBeInTheDocument();
    });
  });
});
