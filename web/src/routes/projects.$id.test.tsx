import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createMemoryHistory } from '@tanstack/react-router';
import { act } from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createAppRouter } from '../router';
import { AuthProvider } from '../auth/AuthContext';

// ─── Test helper ─────────────────────────────────────────────────────────

async function renderAt(path: string) {
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
      <AuthProvider>
        <RouterProvider router={router} />
      </AuthProvider>
    </QueryClientProvider>,
  );
}

// ─── Helpers ─────────────────────────────────────────────────────────────

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

// ─── Fixtures ────────────────────────────────────────────────────────────

const sampleProject = {
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
  pipelines: ['ci', 'cd', 'lint'],
  installation_id: 101,
  created_at: '2026-06-01T00:00:00Z',
  updated_at: '2026-06-01T00:00:00Z',
};

const sampleRules: Array<{
  id: string;
  project_id: string;
  label: string;
  pipeline: string;
  enabled: boolean;
  priority: number;
  created_at: string;
  updated_at: string;
}> = [
  {
    id: 'rule-1',
    project_id: 'proj-1',
    label: 'Auto-deploy main',
    pipeline: 'ci',
    enabled: true,
    priority: 10,
    created_at: '2026-06-01T00:00:00Z',
    updated_at: '2026-06-01T00:00:00Z',
  },
  {
    id: 'rule-2',
    project_id: 'proj-1',
    label: 'PR checks',
    pipeline: 'lint',
    enabled: false,
    priority: 20,
    created_at: '2026-06-02T00:00:00Z',
    updated_at: '2026-06-02T00:00:00Z',
  },
];

// ─── Tests ───────────────────────────────────────────────────────────────

describe('ProjectDetailPage', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    localStorage.setItem('flux_token', 'test-token');
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  // --- Loading state ---

  it('shows loading indicator while fetching project', async () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    await renderAt('/projects/proj-1');

    expect(screen.getByRole('status', { name: /loading/i })).toBeInTheDocument();
  });

  // --- Error state ---

  it('shows error banner on fetch failure', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/v1/projects/proj-1') {
        return Promise.resolve(
          new Response(JSON.stringify({ error: 'Not found' }), {
            status: 404,
            headers: { 'Content-Type': 'application/json' },
          }),
        );
      }
      return Promise.reject(new Error(`Unexpected: ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/not found/i)).toBeInTheDocument();
    });
  });

  it('shows a retry button in the error banner', async () => {
    const user = userEvent.setup();
    let callCount = 0;
    mockFetch.mockImplementation((url: string) => {
      callCount++;
      // First call returns error, subsequent calls return success
      if (callCount <= 2) {
        return Promise.resolve(
          new Response(JSON.stringify({ error: 'boom' }), {
            status: 500,
            headers: { 'Content-Type': 'application/json' },
          }),
        );
      }
      if (url === '/api/v1/projects/proj-1') {
        return Promise.resolve(jsonResponse(sampleProject));
      }
      if (url === '/api/v1/projects/proj-1/trigger-rules') {
        return Promise.resolve(jsonResponse([]));
      }
      return Promise.reject(new Error(`Unexpected: ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    const retryButton = screen.getByRole('button', { name: /retry/i });
    await user.click(retryButton);

    await waitFor(() => {
      expect(screen.getByText('flux-core')).toBeInTheDocument();
    });
  });

  // --- Success: project info ---

  it('renders project name and repo URL', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/v1/projects/proj-1') {
        return Promise.resolve(jsonResponse(sampleProject));
      }
      if (url === '/api/v1/projects/proj-1/trigger-rules') {
        return Promise.resolve(jsonResponse([]));
      }
      return Promise.reject(new Error(`Unexpected: ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      expect(screen.getByText('flux-core')).toBeInTheDocument();
      expect(
        screen.getByText('https://github.com/decko/flux'),
      ).toBeInTheDocument();
    });
  });

  it('renders project created date', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/v1/projects/proj-1') {
        return Promise.resolve(jsonResponse(sampleProject));
      }
      if (url === '/api/v1/projects/proj-1/trigger-rules') {
        return Promise.resolve(jsonResponse([]));
      }
      return Promise.reject(new Error(`Unexpected: ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      expect(screen.getByText(/created/i)).toBeInTheDocument();
      // Date might show as May 31 or Jun 1 depending on timezone
      expect(screen.getByText(/2026/i)).toBeInTheDocument();
    });
  });

  // --- Pipelines ---

  it('renders pipeline list', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/v1/projects/proj-1') {
        return Promise.resolve(jsonResponse(sampleProject));
      }
      if (url === '/api/v1/projects/proj-1/trigger-rules') {
        return Promise.resolve(jsonResponse([]));
      }
      return Promise.reject(new Error(`Unexpected: ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      // Pipeline names appear both in the pipeline list and the add-form dropdown
      expect(screen.getAllByText('ci').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('cd').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('lint').length).toBeGreaterThanOrEqual(1);
    });
  });

  it('shows "No pipelines configured" when pipelines list is empty', async () => {
    const projectNoPipelines = { ...sampleProject, pipelines: [] };
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/v1/projects/proj-1') {
        return Promise.resolve(jsonResponse(projectNoPipelines));
      }
      if (url === '/api/v1/projects/proj-1/trigger-rules') {
        return Promise.resolve(jsonResponse([]));
      }
      return Promise.reject(new Error(`Unexpected: ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      expect(
        screen.getByText(/no pipelines configured/i),
      ).toBeInTheDocument();
    });
  });

  // --- Trigger rules ---

  it('renders trigger rules', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/v1/projects/proj-1') {
        return Promise.resolve(jsonResponse(sampleProject));
      }
      if (url === '/api/v1/projects/proj-1/trigger-rules') {
        return Promise.resolve(jsonResponse(sampleRules));
      }
      return Promise.reject(new Error(`Unexpected: ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      expect(screen.getByText('Auto-deploy main')).toBeInTheDocument();
      expect(screen.getByText('PR checks')).toBeInTheDocument();
    });
  });

  it('shows pipeline names in trigger rules', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/v1/projects/proj-1') {
        return Promise.resolve(jsonResponse(sampleProject));
      }
      if (url === '/api/v1/projects/proj-1/trigger-rules') {
        return Promise.resolve(jsonResponse(sampleRules));
      }
      return Promise.reject(new Error(`Unexpected: ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      // Rule labels: Auto-deploy main uses ci, PR checks uses lint
      expect(screen.getByText('Auto-deploy main')).toBeInTheDocument();
      expect(screen.getByText('PR checks')).toBeInTheDocument();
      // Pipeline names appear in rules and in dropdown
      expect(screen.getAllByText('ci').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('lint').length).toBeGreaterThanOrEqual(1);
    });
  });

  it('renders empty state when no trigger rules exist', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/v1/projects/proj-1') {
        return Promise.resolve(jsonResponse(sampleProject));
      }
      if (url === '/api/v1/projects/proj-1/trigger-rules') {
        return Promise.resolve(jsonResponse([]));
      }
      return Promise.reject(new Error(`Unexpected: ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      expect(
        screen.getByText(/no trigger rules configured/i),
      ).toBeInTheDocument();
    });
  });

  // --- CRUD: Add ---

  it('adds a new trigger rule', async () => {
    const user = userEvent.setup();
    let postedBody: unknown = null;

    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/projects/proj-1' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleProject));
      }
      if (url === '/api/v1/projects/proj-1/trigger-rules' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleRules));
      }
      if (
        url === '/api/v1/projects/proj-1/trigger-rules' &&
        method === 'POST'
      ) {
        postedBody = JSON.parse((options?.body as string) ?? '{}');
        return Promise.resolve(
          jsonResponse(
            {
              id: 'rule-3',
              project_id: 'proj-1',
              ...(postedBody as Record<string, unknown>),
              enabled: true,
              priority: 30,
              created_at: '2026-06-26T00:00:00Z',
              updated_at: '2026-06-26T00:00:00Z',
            },
            201,
          ),
        );
      }
      return Promise.reject(new Error(`Unexpected: ${method} ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      expect(screen.getByText('Add Trigger Rule')).toBeInTheDocument();
    });

    const labelInput = screen.getByPlaceholderText(/e\.g\. auto-deploy main/i);
    await user.type(labelInput, 'Nightly build');

    const saveButton = screen.getByRole('button', { name: /save/i });
    await user.click(saveButton);

    await waitFor(() => {
      expect(postedBody).toEqual({ label: 'Nightly build', pipeline: 'ci' });
    });
  });

  // --- CRUD: Edit ---

  it('allows editing a trigger rule label and pipeline', async () => {
    const user = userEvent.setup();
    let updatedBody: unknown = null;

    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/projects/proj-1' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleProject));
      }
      if (url === '/api/v1/projects/proj-1/trigger-rules' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleRules));
      }
      if (
        url === '/api/v1/projects/proj-1/trigger-rules/rule-1' &&
        method === 'PUT'
      ) {
        updatedBody = JSON.parse((options?.body as string) ?? '{}');
        return Promise.resolve(jsonResponse({ ...sampleRules[0], ...(updatedBody as Record<string, unknown>) }));
      }
      return Promise.reject(new Error(`Unexpected: ${method} ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      expect(screen.getByText('Auto-deploy main')).toBeInTheDocument();
    });

    // Click Edit on the first rule
    const editButtons = screen.getAllByText('Edit');
    await user.click(editButtons[0]!);

    // The inline editor should appear
    const labelInput = screen.getByDisplayValue('Auto-deploy main');
    await user.clear(labelInput);
    await user.type(labelInput, 'Updated rule');

    const saveButtons = screen.getAllByText('Save');
    await user.click(saveButtons[0]!);

    await waitFor(() => {
      expect(updatedBody).toEqual({
        label: 'Updated rule',
        pipeline: 'ci',
      });
    });
  });

  // --- CRUD: Toggle ---

  it('toggles a trigger rule enabled state', async () => {
    const user = userEvent.setup();
    let toggledBody: unknown = null;

    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/projects/proj-1' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleProject));
      }
      if (url === '/api/v1/projects/proj-1/trigger-rules' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleRules));
      }
      if (
        url === '/api/v1/projects/proj-1/trigger-rules/rule-2' &&
        method === 'PUT'
      ) {
        toggledBody = JSON.parse((options?.body as string) ?? '{}');
        return Promise.resolve(jsonResponse({ ...sampleRules[1], ...(toggledBody as Record<string, unknown>) }));
      }
      return Promise.reject(new Error(`Unexpected: ${method} ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      expect(screen.getByText('PR checks')).toBeInTheDocument();
    });

    // Find the checkbox for the second rule (PR checks, currently disabled)
    const checkboxes = screen.getAllByRole('checkbox');
    expect(checkboxes).toHaveLength(2);

    // Click the second checkbox (PR checks, enabled=false -> enable it)
    await user.click(checkboxes[1]!);

    await waitFor(() => {
      expect(toggledBody).toEqual({ enabled: true });
    });
  });

  // --- CRUD: Delete ---

  it('deletes a trigger rule', async () => {
    const user = userEvent.setup();
    let deletedRuleId: string | null = null;

    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/projects/proj-1' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleProject));
      }
      if (url === '/api/v1/projects/proj-1/trigger-rules' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleRules));
      }
      if (
        url === '/api/v1/projects/proj-1/trigger-rules/rule-2' &&
        method === 'DELETE'
      ) {
        deletedRuleId = 'rule-2';
        return Promise.resolve(jsonResponse(null, 204));
      }
      return Promise.reject(new Error(`Unexpected: ${method} ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      expect(screen.getByText('PR checks')).toBeInTheDocument();
    });

    // Click Delete on the second rule
    const deleteButtons = screen.getAllByText('Delete');
    expect(deleteButtons).toHaveLength(2);
    await user.click(deleteButtons[1]!);

    await waitFor(() => {
      expect(deletedRuleId).toBe('rule-2');
    });
  });

  // --- Navigation ---

  it('has a back button that navigates to /projects', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/v1/projects/proj-1') {
        return Promise.resolve(jsonResponse(sampleProject));
      }
      if (url === '/api/v1/projects/proj-1/trigger-rules') {
        return Promise.resolve(jsonResponse([]));
      }
      return Promise.reject(new Error(`Unexpected: ${url}`));
    });

    await renderAt('/projects/proj-1');

    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /back to projects/i }),
      ).toBeInTheDocument();
    });
  });

  // --- Auth ---

  it('redirects to login when no token is present', async () => {
    localStorage.removeItem('flux_token');

    // Mock fetch to never be called (redirect happens before load)
    mockFetch.mockRejectedValue(new Error('should not fetch'));

    await renderAt('/projects/proj-1');

    // Should have redirected to /login
    await waitFor(() => {
      expect(
        screen.getByRole('heading', { name: /sign in/i }),
      ).toBeInTheDocument();
    });
  });
});
