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
    installation_id: 101,
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
    installation_id: 202,
    created_at: '2026-06-02T00:00:00Z',
    updated_at: '2026-06-02T00:00:00Z',
  },
];

// ---- Wizard test fixtures ----

const sampleInstallations = [
  {
    id: 101,
    account: { login: 'flux-org' },
    target_type: 'Organization',
    html_url: 'https://github.com/flux-org',
  },
  {
    id: 202,
    account: { login: 'decko' },
    target_type: 'User',
    html_url: 'https://github.com/decko',
  },
];

const sampleRepos = [
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

describe('ProjectsPage', () => {
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

  it('shows loading skeletons while projects are being fetched', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPage();

    const skeletons = screen.getAllByRole('status', { name: /loading/i });
    expect(skeletons).toHaveLength(2);
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
    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/github/installations' && method === 'GET') {
        return Promise.resolve(jsonResponse([]));
      }
      if (url === '/api/v1/projects' && method === 'GET') {
        return Promise.resolve(jsonResponse([]));
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/no projects/i)).toBeInTheDocument();
    });
  });

  // --- Error state ---

  it('shows an error banner when the fetch fails', async () => {
    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/github/installations' && method === 'GET') {
        return Promise.resolve(jsonErrorResponse(500, 'Internal Server Error'));
      }
      if (url === '/api/v1/projects' && method === 'GET') {
        return Promise.resolve(jsonErrorResponse(500, 'Internal Server Error'));
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    renderPage();

    await waitFor(() => {
      const alerts = screen.getAllByRole('alert');
      expect(alerts).toHaveLength(2);
      expect(screen.getAllByText(/internal server error/i)).toHaveLength(2);
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
    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/github/installations' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleInstallations));
      }
      if (url === '/api/v1/projects' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleProjects));
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('flux-core')).toBeInTheDocument();
      expect(screen.getByText('web-app')).toBeInTheDocument();
    });
  });

  it('renders the correct number of project cards', async () => {
    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/github/installations' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleInstallations));
      }
      if (url === '/api/v1/projects' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleProjects));
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    renderPage();

    await waitFor(() => {
      const cards = screen.getAllByTestId('project-card');
      expect(cards).toHaveLength(sampleProjects.length);
    });
  });

  it('displays the repo URL for each project', async () => {
    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/github/installations' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleInstallations));
      }
      if (url === '/api/v1/projects' && method === 'GET') {
        return Promise.resolve(jsonResponse([sampleProjects[0]]));
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('https://github.com/decko/flux')).toBeInTheDocument();
    });
  });

  // --- Create wizard ---

  it('renders the create project form', () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    renderPage();

    expect(screen.getByText('Create Project')).toBeInTheDocument();
    expect(screen.getByText('Step 1: Select a GitHub App installation')).toBeInTheDocument();
  });

  it('sends a POST request with name and repo_url when creating a project', async () => {
    const user = userEvent.setup();

    const createdProject = {
      id: 'proj-3',
      name: 'flux',
      repo_url: 'https://github.com/flux-org/flux',
      definition: { language: '', framework: '', conventions: [], architecture: '' },
      adapters: [],
      pipelines: [],
      installation_id: 101,
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    };

    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/github/installations' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleInstallations));
      }
      if (url === '/api/v1/github/installations/101/repositories' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleRepos));
      }
      if (url === '/api/v1/projects' && method === 'GET') {
        return Promise.resolve(jsonResponse([]));
      }
      if (url === '/api/v1/projects' && method === 'POST') {
        return Promise.resolve(jsonResponse(createdProject, 201));
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    renderPage();

    // Step 1: select installation
    await waitFor(() => {
      expect(screen.getByText('flux-org')).toBeInTheDocument();
    });
    await user.click(screen.getByText('flux-org'));

    // Step 2: select repository
    await waitFor(() => {
      expect(screen.getByText('flux')).toBeInTheDocument();
    });
    await user.click(screen.getByText('flux'));

    // Step 3: confirm and create
    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /create project/i }),
      ).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: /create project/i }));

    // Verify POST body includes name, repo_url, and installation_id
    await waitFor(() => {
      const postCall = mockFetch.mock.calls.find(
        ([url, opts]) =>
          url === '/api/v1/projects' &&
          (opts as RequestInit)?.method === 'POST',
      );
      expect(postCall).toBeDefined();
      if (postCall) {
        const body = JSON.parse(
          (postCall[1] as RequestInit).body as string,
        );
        expect(body).toEqual({
          name: 'flux',
          repo_url: 'https://github.com/flux-org/flux',
          installation_id: 101,
        });
      }
    });
  });

  it('shows the newly created project in the list', async () => {
    const user = userEvent.setup();

    const newProject = {
      id: 'proj-3',
      name: 'flux',
      repo_url: 'https://github.com/flux-org/flux',
      definition: { language: '', framework: '', conventions: [], architecture: '' },
      adapters: [],
      pipelines: [],
      installation_id: 101,
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    };

    let projectsCalled = 0;
    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/github/installations' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleInstallations));
      }
      if (url === '/api/v1/github/installations/101/repositories' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleRepos));
      }
      if (url === '/api/v1/projects' && method === 'GET') {
        projectsCalled++;
        if (projectsCalled === 1) {
          return Promise.resolve(jsonResponse([]));
        }
        return Promise.resolve(jsonResponse([newProject]));
      }
      if (url === '/api/v1/projects' && method === 'POST') {
        return Promise.resolve(jsonResponse(newProject, 201));
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    renderPage();

    // Step 1: select installation
    await waitFor(() => {
      expect(screen.getByText('flux-org')).toBeInTheDocument();
    });
    await user.click(screen.getByText('flux-org'));

    // Step 2: select repository
    await waitFor(() => {
      expect(screen.getByText('flux')).toBeInTheDocument();
    });
    await user.click(screen.getByText('flux'));

    // Step 3: create project
    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /create project/i }),
      ).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: /create project/i }));

    // After creation, wizard resets to step 1 and shows project list
    await waitFor(() => {
      const cards = screen.getAllByTestId('project-card');
      expect(cards).toHaveLength(1);
      expect(screen.getByText('flux')).toBeInTheDocument();
    });
  });
});

// --- Project creation wizard tests (Issue #156) ---
// These tests exercise the new 3-step wizard flow that replaces the
// manual create form.

describe('Project creation wizard', () => {
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

  describe('Step 1 — Installation Picker', () => {
    it('renders installation picker as step 1', async () => {
      mockFetch.mockImplementation((url: string, options?: RequestInit) => {
        const method = options?.method || 'GET';
        if (url === '/api/v1/github/installations' && method === 'GET') {
          return Promise.resolve(jsonResponse(sampleInstallations));
        }
        if (url === '/api/v1/projects' && method === 'GET') {
          return Promise.resolve(jsonResponse([]));
        }
        return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
      });

      renderPage();

      await waitFor(() => {
        expect(screen.getByText('flux-org')).toBeInTheDocument();
      });
      expect(screen.getByText('decko')).toBeInTheDocument();
    });
  });

  describe('Step 2 — Repository Picker', () => {
    it('advances to step 2 after selecting installation', async () => {
      const user = userEvent.setup();
      mockFetch.mockImplementation((url: string, options?: RequestInit) => {
        const method = options?.method || 'GET';
        if (url === '/api/v1/github/installations' && method === 'GET') {
          return Promise.resolve(jsonResponse(sampleInstallations));
        }
        if (
          url === '/api/v1/github/installations/101/repositories' &&
          method === 'GET'
        ) {
          return Promise.resolve(jsonResponse(sampleRepos));
        }
        if (url === '/api/v1/projects' && method === 'GET') {
          return Promise.resolve(jsonResponse([]));
        }
        return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
      });

      renderPage();

      await waitFor(() => {
        expect(screen.getByText('flux-org')).toBeInTheDocument();
      });

      await user.click(screen.getByText('flux-org'));

      // Should now show RepositoryPicker content
      await waitFor(() => {
        expect(screen.getByText('flux')).toBeInTheDocument();
      });
    });
  });

  describe('Step 3 — Confirm & Create', () => {
    it('advances to step 3 after selecting repository', async () => {
      const user = userEvent.setup();
      mockFetch.mockImplementation((url: string, options?: RequestInit) => {
        const method = options?.method || 'GET';
        if (url === '/api/v1/github/installations' && method === 'GET') {
          return Promise.resolve(jsonResponse(sampleInstallations));
        }
        if (
          url === '/api/v1/github/installations/101/repositories' &&
          method === 'GET'
        ) {
          return Promise.resolve(jsonResponse(sampleRepos));
        }
        if (url === '/api/v1/projects' && method === 'GET') {
          return Promise.resolve(jsonResponse([]));
        }
        return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
      });

      renderPage();

      // Step 1: select installation
      await waitFor(() => {
        expect(screen.getByText('flux-org')).toBeInTheDocument();
      });
      await user.click(screen.getByText('flux-org'));

      // Step 2: select repository
      await waitFor(() => {
        expect(screen.getByText('flux')).toBeInTheDocument();
      });
      await user.click(screen.getByText('flux'));

      // Step 3: confirmation step shows repo name, url, and Create button
      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /create project/i }),
        ).toBeInTheDocument();
      });
      expect(screen.getByText('flux')).toBeInTheDocument();
      expect(
        screen.getByText('https://github.com/flux-org/flux'),
      ).toBeInTheDocument();
    });

    it('creates project with installation_id on confirm', async () => {
      const user = userEvent.setup();
      const createdProject = {
        id: 'proj-wizard',
        name: 'flux',
        repo_url: 'https://github.com/flux-org/flux',
        definition: {
          language: '',
          framework: '',
          conventions: [] as string[],
          architecture: '',
        },
        adapters: [] as unknown[],
        pipelines: [] as unknown[],
        installation_id: 101,
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      };

      mockFetch.mockImplementation((url: string, options?: RequestInit) => {
        const method = options?.method || 'GET';
        if (url === '/api/v1/github/installations' && method === 'GET') {
          return Promise.resolve(jsonResponse(sampleInstallations));
        }
        if (
          url === '/api/v1/github/installations/101/repositories' &&
          method === 'GET'
        ) {
          return Promise.resolve(jsonResponse(sampleRepos));
        }
        if (url === '/api/v1/projects' && method === 'GET') {
          return Promise.resolve(jsonResponse([createdProject]));
        }
        if (url === '/api/v1/projects' && method === 'POST') {
          return Promise.resolve(jsonResponse(createdProject, 201));
        }
        return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
      });

      renderPage();

      // Step 1: select installation
      await waitFor(() => {
        expect(screen.getByText('flux-org')).toBeInTheDocument();
      });
      await user.click(screen.getByText('flux-org'));

      // Step 2: select repository
      await waitFor(() => {
        expect(screen.getByText('flux')).toBeInTheDocument();
      });
      await user.click(screen.getByText('flux'));

      // Step 3: click Create Project
      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /create project/i }),
        ).toBeInTheDocument();
      });
      await user.click(screen.getByRole('button', { name: /create project/i }));

      // Verify the POST body includes installation_id
      await waitFor(() => {
        const postCall = mockFetch.mock.calls.find(
          ([url, opts]) =>
            url === '/api/v1/projects' &&
            (opts as RequestInit)?.method === 'POST',
        );
        expect(postCall).toBeDefined();
        if (postCall) {
          const body = JSON.parse(
            (postCall[1] as RequestInit).body as string,
          );
          expect(body).toEqual({
            name: 'flux',
            repo_url: 'https://github.com/flux-org/flux',
            installation_id: 101,
          });
        }
      });
    });
  });

  describe('Navigation', () => {
    it('back button returns to previous step', async () => {
      const user = userEvent.setup();
      mockFetch.mockImplementation((url: string, options?: RequestInit) => {
        const method = options?.method || 'GET';
        if (url === '/api/v1/github/installations' && method === 'GET') {
          return Promise.resolve(jsonResponse(sampleInstallations));
        }
        if (
          url === '/api/v1/github/installations/101/repositories' &&
          method === 'GET'
        ) {
          return Promise.resolve(jsonResponse(sampleRepos));
        }
        if (url === '/api/v1/projects' && method === 'GET') {
          return Promise.resolve(jsonResponse([]));
        }
        return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
      });

      renderPage();

      // Step 1 → Step 2
      await waitFor(() => {
        expect(screen.getByText('flux-org')).toBeInTheDocument();
      });
      await user.click(screen.getByText('flux-org'));

      // Confirm we're on step 2
      await waitFor(() => {
        expect(screen.getByText('flux')).toBeInTheDocument();
      });

      // Click Back → should return to step 1
      await user.click(screen.getByRole('button', { name: /back/i }));

      await waitFor(() => {
        expect(screen.getByText('flux-org')).toBeInTheDocument();
        expect(screen.getByText('decko')).toBeInTheDocument();
      });
    });
  });

  describe('Error handling', () => {
    it('shows GitHub App not configured when installations fail with 503', async () => {
      mockFetch.mockImplementation((url: string, options?: RequestInit) => {
        const method = options?.method || 'GET';
        if (url === '/api/v1/github/installations' && method === 'GET') {
          return Promise.resolve(
            jsonErrorResponse(503, 'GitHub App not configured'),
          );
        }
        if (url === '/api/v1/projects' && method === 'GET') {
          return Promise.resolve(jsonResponse([]));
        }
        return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
      });

      renderPage();

      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument();
        expect(
          screen.getByText(/github app not configured/i),
        ).toBeInTheDocument();
      });
    });

    it('handles creation error', async () => {
      const user = userEvent.setup();
      mockFetch.mockImplementation((url: string, options?: RequestInit) => {
        const method = options?.method || 'GET';
        if (url === '/api/v1/github/installations' && method === 'GET') {
          return Promise.resolve(jsonResponse(sampleInstallations));
        }
        if (
          url === '/api/v1/github/installations/101/repositories' &&
          method === 'GET'
        ) {
          return Promise.resolve(jsonResponse(sampleRepos));
        }
        if (url === '/api/v1/projects' && method === 'GET') {
          return Promise.resolve(jsonResponse([]));
        }
        if (url === '/api/v1/projects' && method === 'POST') {
          return Promise.resolve(
            jsonErrorResponse(500, 'Failed to create project'),
          );
        }
        return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
      });

      renderPage();

      // Navigate through all 3 steps
      await waitFor(() => {
        expect(screen.getByText('flux-org')).toBeInTheDocument();
      });
      await user.click(screen.getByText('flux-org'));

      await waitFor(() => {
        expect(screen.getByText('flux')).toBeInTheDocument();
      });
      await user.click(screen.getByText('flux'));

      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /create project/i }),
        ).toBeInTheDocument();
      });
      await user.click(screen.getByRole('button', { name: /create project/i }));

      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument();
        expect(
          screen.getByText(/failed to create project/i),
        ).toBeInTheDocument();
      });
    });
  });

  describe('Project list integration', () => {
    it('existing project list still renders alongside wizard', async () => {
      mockFetch.mockImplementation((url: string, options?: RequestInit) => {
        const method = options?.method || 'GET';
        if (url === '/api/v1/github/installations' && method === 'GET') {
          return Promise.resolve(jsonResponse(sampleInstallations));
        }
        if (url === '/api/v1/projects' && method === 'GET') {
          return Promise.resolve(jsonResponse(sampleProjects));
        }
        return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
      });

      renderPage();

      // The wizard should show installation picker
      await waitFor(() => {
        expect(screen.getByText('flux-org')).toBeInTheDocument();
      });

      // The project list should still render below the wizard
      const cards = screen.getAllByTestId('project-card');
      expect(cards).toHaveLength(sampleProjects.length);
      expect(screen.getByText('flux-core')).toBeInTheDocument();
    });
  });
});
