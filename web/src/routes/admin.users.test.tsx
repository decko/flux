import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createMemoryHistory } from '@tanstack/react-router';
import { act } from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createAppRouter } from '../router';
import { AuthProvider } from '../auth/AuthContext';

// ─── Integration test helper ─────────────────────────────────────────────

async function renderAdminPage(path = '/admin/users') {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  const memoryHistory = createMemoryHistory({ initialEntries: [path] });
  const appRouter = createAppRouter(memoryHistory);

  await act(async () => {
    await appRouter.load();
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <RouterProvider router={appRouter} />
      </AuthProvider>
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

function jsonErrorResponse(status: number, message: string): Response {
  return new Response(JSON.stringify({ error: message }), {
    status,
    statusText: message,
    headers: { 'Content-Type': 'application/json' },
  });
}

// ─── JWT helper for test tokens ─────────────────────────────────────────

/**
 * Create a fake JWT with the given payload.
 * The signature is dummy — tests don't verify cryptographic validity.
 */
function makeToken(payload: Record<string, unknown>): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const body = btoa(JSON.stringify(payload));
  return `${header}.${body}.fake-signature`;
}

const ADMIN_TOKEN = makeToken({ sub: 'u1', email: 'admin@flux.dev', role: 'admin' });
const USER_TOKEN = makeToken({ sub: 'u2', email: 'dev@flux.dev', role: 'user' });

// ─── Fixtures ───────────────────────────────────────────────────────────

const sampleUsers = [
  {
    id: 'user-1',
    email: 'admin@flux.dev',
    role: 'admin',
    created_at: '2026-01-01T00:00:00Z',
  },
  {
    id: 'user-2',
    email: 'dev@flux.dev',
    role: 'user',
    created_at: '2026-01-02T00:00:00Z',
  },
];

// ─── Tests ──────────────────────────────────────────────────────────────

describe('AdminUsersPage', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    localStorage.setItem('flux_token', ADMIN_TOKEN);
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  // ── Loading state ───────────────────────────────────────────────────

  it('renders loading skeleton while users are being fetched', async () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    await renderAdminPage();

    expect(screen.getByRole('status', { name: /loading users/i })).toBeInTheDocument();
  });

  it('does not show empty state while still loading', async () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    await renderAdminPage();

    expect(screen.queryByText(/no users found/i)).toBeNull();
  });

  it('does not show error state while still loading', async () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    await renderAdminPage();

    expect(screen.queryByRole('alert')).toBeNull();
  });

  // ── Success state: user table ───────────────────────────────────────

  it('renders user emails when data is loaded', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleUsers));

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByText('admin@flux.dev')).toBeInTheDocument();
      expect(screen.getByText('dev@flux.dev')).toBeInTheDocument();
    });
  });

  it('renders role badges for each user', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleUsers));

    await renderAdminPage();

    await waitFor(() => {
      // Each user has a role badge indicating "admin" or "user"
      const adminBadges = screen.getAllByText('admin');
      const userBadges = screen.getAllByText('user');
      expect(adminBadges.length).toBeGreaterThanOrEqual(1);
      expect(userBadges.length).toBeGreaterThanOrEqual(1);
    });
  });

  it('renders the page heading', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleUsers));

    await renderAdminPage();

    await waitFor(() => {
      expect(
        screen.getByRole('heading', { name: /user management/i }),
      ).toBeInTheDocument();
    });
  });

  // ── Empty state ─────────────────────────────────────────────────────

  it('shows empty state when there are no users', async () => {
    mockFetch.mockResolvedValue(jsonResponse([]));

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByText(/no users found/i)).toBeInTheDocument();
    });
  });

  it('still shows Add User button when user list is empty', async () => {
    mockFetch.mockResolvedValue(jsonResponse([]));

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByText(/no users found/i)).toBeInTheDocument();
    });

    // Add User button should still be visible even with empty list
    expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
  });

  // ── Error state ─────────────────────────────────────────────────────

  it('shows error banner when fetch fails', async () => {
    mockFetch.mockResolvedValue(jsonErrorResponse(500, 'Internal Server Error'));

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/internal server error/i)).toBeInTheDocument();
    });
  });

  it('shows error banner on network failure', async () => {
    mockFetch.mockRejectedValue(new Error('Network Error'));

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/network error/i)).toBeInTheDocument();
    });
  });

  // ── Change user role ────────────────────────────────────────────────

  it('changes a user role via dropdown and shows optimistic update', async () => {
    const user = userEvent.setup();

    const userToUpdate = sampleUsers[1];
    if (!userToUpdate) throw new Error('Test fixture error: sampleUsers[1] is undefined');
    const updatedUser = {
      ...userToUpdate,
      role: 'admin',
    };

    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/admin/users' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleUsers));
      }
      if (url === '/api/v1/admin/users/user-2/role' && method === 'PUT') {
        return Promise.resolve(jsonResponse(updatedUser));
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    await renderAdminPage();

    // Wait for users to load
    await waitFor(() => {
      expect(screen.getByText('dev@flux.dev')).toBeInTheDocument();
    });

    // Find the role dropdown/select for dev@flux.dev and change to "admin"
    const roleSelect = screen.getByLabelText(/role for dev@flux.dev/i);
    await user.selectOptions(roleSelect, 'admin');

    // Verify the PUT request was sent with correct body
    await waitFor(() => {
      const putCall = mockFetch.mock.calls.find(
        ([url, opts]) =>
          url === '/api/v1/admin/users/user-2/role' &&
          (opts as RequestInit)?.method === 'PUT',
      );
      expect(putCall).toBeDefined();
      if (putCall) {
        const body = JSON.parse((putCall[1] as RequestInit).body as string);
        expect(body).toEqual({ role: 'admin' });
      }
    });

    // Verify optimistic update: the user's role should now show "admin"
    await waitFor(() => {
      // dev@flux.dev should now have an "admin" badge
      // After optimistic update, both users show admin role
      const adminBadges = screen.getAllByText('admin');
      expect(adminBadges.length).toBeGreaterThanOrEqual(2);
    });
  });

  // ── Delete user ─────────────────────────────────────────────────────

  it('deletes a user after confirmation', async () => {
    const user = userEvent.setup();

    // Return 2 users initially, then 1 user after delete
    let usersReturned = sampleUsers;
    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/admin/users' && method === 'GET') {
        return Promise.resolve(jsonResponse(usersReturned));
      }
      if (url === '/api/v1/admin/users/user-2' && method === 'DELETE') {
        const firstUser = sampleUsers[0];
        if (!firstUser) throw new Error('Test fixture error: sampleUsers[0] is undefined');
        usersReturned = [firstUser]; // remove user-2
        return Promise.resolve(new Response(null, { status: 204 }));
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    await renderAdminPage();

    // Wait for users to load
    await waitFor(() => {
      expect(screen.getByText('dev@flux.dev')).toBeInTheDocument();
    });

    // Click delete button for dev@flux.dev
    const deleteButton = screen.getByRole('button', {
      name: /delete dev@flux.dev/i,
    });
    await user.click(deleteButton);

    // Confirm deletion in the dialog
    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /confirm delete/i }),
      ).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: /confirm delete/i }));

    // Verify the DELETE request was sent
    await waitFor(() => {
      const deleteCall = mockFetch.mock.calls.find(
        ([url, opts]) =>
          url === '/api/v1/admin/users/user-2' &&
          (opts as RequestInit)?.method === 'DELETE',
      );
      expect(deleteCall).toBeDefined();
    });

    // Verify user is removed from the list
    await waitFor(() => {
      expect(screen.queryByText('dev@flux.dev')).not.toBeInTheDocument();
      // admin@flux.dev should still be visible
      expect(screen.getByText('admin@flux.dev')).toBeInTheDocument();
    });
  });

  // ── Access denied for non-admin ─────────────────────────────────────

  it('shows access denied for non-admin user', async () => {
    // Override token with a non-admin JWT
    localStorage.setItem('flux_token', USER_TOKEN);
    // Don't mock fetch — the page should reject before making API calls
    mockFetch.mockImplementation(() =>
      Promise.reject(new Error('Fetch should not be called for non-admin')),
    );

    await renderAdminPage();

    await waitFor(() => {
      expect(
        screen.getByText(/access denied/i),
      ).toBeInTheDocument();
    });

    // Verify no user data fetch was attempted
    expect(mockFetch).not.toHaveBeenCalled();
  });

  // ── Redirect to login when unauthenticated ──────────────────────────

  it('redirects to login page when not authenticated', async () => {
    // Clear token to simulate unauthenticated state
    localStorage.clear();

    await renderAdminPage();

    // Should be redirected to login — sign in heading should be visible
    await waitFor(() => {
      expect(
        screen.getByRole('heading', { name: /sign in/i }),
      ).toBeInTheDocument();
    });

    // Admin page content should not be visible
    expect(screen.queryByText(/admin users/i)).not.toBeInTheDocument();
  });

  // ── Create User: rendering ────────────────────────────────────────────

  it('renders "Add User" button above the users table', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleUsers));

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
    });
  });

  // ── Create User: form appears on click ────────────────────────────────

  it('shows create user form when "Add User" button is clicked', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValue(jsonResponse(sampleUsers));

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /add user/i }));

    // Form fields should appear in the modal
    await waitFor(() => {
      expect(screen.getByLabelText('Email')).toBeInTheDocument();
      expect(screen.getByLabelText('Password')).toBeInTheDocument();
      expect(screen.getByLabelText('Confirm password')).toBeInTheDocument();
      // Use exact match to avoid conflicts with table role selects
      const roleSelects = screen.getAllByLabelText('Role');
      expect(roleSelects.length).toBeGreaterThanOrEqual(1);
    });
  });

  // ── Create User: generate password ────────────────────────────────────

  it('generates a random password when "Generate" button is clicked', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValue(jsonResponse(sampleUsers));

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /add user/i }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /generate/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /generate/i }));

    // Password fields should be populated (non-empty, at least 16 chars)
    await waitFor(() => {
      const pwInput = screen.getByLabelText(/^password$/i) as HTMLInputElement;
      const confirmInput = screen.getByLabelText(/confirm password/i) as HTMLInputElement;
      expect(pwInput.value.length).toBeGreaterThanOrEqual(16);
      expect(confirmInput.value.length).toBeGreaterThanOrEqual(16);
    });
  });

  // ── Create User: successful submission ────────────────────────────────

  it('creates a new user and sends correct POST body', async () => {
    const user = userEvent.setup();
    const newUser = {
      id: 'new-1',
      email: 'new@flux.dev',
      role: 'user',
      created_at: '2026-06-01T00:00:00Z',
    };

    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/admin/users' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleUsers));
      }
      if (url === '/api/v1/admin/users' && method === 'POST') {
        return Promise.resolve(jsonResponse(newUser, 201));
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /add user/i }));

    await waitFor(() => {
      expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText(/email/i), 'new@flux.dev');
    await user.type(screen.getByLabelText(/^password$/i), 'securepass1234');
    await user.type(screen.getByLabelText(/confirm password/i), 'securepass1234');
    await user.click(screen.getByRole('button', { name: /create/i }));

    // Verify POST was sent with correct body
    await waitFor(() => {
      const postCall = mockFetch.mock.calls.find(
        ([url, opts]) =>
          url === '/api/v1/admin/users' &&
          (opts as RequestInit)?.method === 'POST',
      );
      expect(postCall).toBeDefined();
      if (postCall) {
        const body = JSON.parse((postCall[1] as RequestInit).body as string);
        expect(body.email).toBe('new@flux.dev');
        expect(body.password).toBe('securepass1234');
        expect(body.role).toBe('user');
      }
    });
  });

  // ── Create User: error handling ───────────────────────────────────────

  it('shows error when create user fails', async () => {
    const user = userEvent.setup();

    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/admin/users' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleUsers));
      }
      if (url === '/api/v1/admin/users' && method === 'POST') {
        return Promise.resolve(jsonErrorResponse(409, 'email already exists'));
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /add user/i }));

    await waitFor(() => {
      expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText(/email/i), 'dupe@flux.dev');
    await user.type(screen.getByLabelText(/^password$/i), 'securepass1234');
    await user.type(screen.getByLabelText(/confirm password/i), 'securepass1234');
    await user.click(screen.getByRole('button', { name: /create/i }));

    await waitFor(() => {
      expect(screen.getByText(/email already exists/i)).toBeInTheDocument();
    });
  });

  // ── Create User: password mismatch ────────────────────────────────────

  it('prevents submission when passwords do not match', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValue(jsonResponse(sampleUsers));

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /add user/i }));

    await waitFor(() => {
      expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText(/email/i), 'new@flux.dev');
    await user.type(screen.getByLabelText(/^password$/i), 'password123456');
    await user.type(screen.getByLabelText(/confirm password/i), 'different1234');
    await user.click(screen.getByRole('button', { name: /create/i }));

    // Should show mismatch error and not send POST
    await waitFor(() => {
      expect(screen.getByText(/passwords do not match/i)).toBeInTheDocument();
    });
  });

  // ── Reset Password: button appears per row ────────────────────────────

  it('renders "Reset Password" button for each user row', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleUsers));

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByText('admin@flux.dev')).toBeInTheDocument();
    });

    // Each user row should have a reset password button
    const resetButtons = screen.getAllByRole('button', { name: /reset password/i });
    expect(resetButtons.length).toBe(2);
  });

  // ── Reset Password: modal opens ───────────────────────────────────────

  it('opens reset password modal when "Reset Password" button is clicked', async () => {
    const user = userEvent.setup();
    mockFetch.mockResolvedValue(jsonResponse(sampleUsers));

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByText('dev@flux.dev')).toBeInTheDocument();
    });

    // Click reset password for dev@flux.dev (second user)
    const resetButtons = screen.getAllByRole('button', { name: /reset password/i });
    expect(resetButtons.length).toBeGreaterThanOrEqual(2);
    await user.click(resetButtons[1]!);

    // Modal should show new password and confirm fields
    await waitFor(() => {
      expect(screen.getByLabelText(/^new password$/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/^confirm/i)).toBeInTheDocument();
    });
  });

  // ── Reset Password: successful submission ─────────────────────────────

  it('resets password successfully via modal', async () => {
    const user = userEvent.setup();

    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/admin/users' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleUsers));
      }
      if (
        url.startsWith('/api/v1/admin/users/') &&
        url.endsWith('/password') &&
        method === 'PUT'
      ) {
        return Promise.resolve(
          jsonResponse({
            id: 'user-2',
            email: 'dev@flux.dev',
            role: 'user',
            created_at: '2026-01-02T00:00:00Z',
          }),
        );
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    await renderAdminPage();

    await waitFor(() => {
      expect(screen.getByText('dev@flux.dev')).toBeInTheDocument();
    });

    const resetButtons = screen.getAllByRole('button', { name: /reset password/i });
    await user.click(resetButtons[1]!);

    await waitFor(() => {
      expect(screen.getByLabelText(/^new password$/i)).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText(/^new password$/i), 'newsecurepass12');
    await user.type(screen.getByLabelText(/^confirm/i), 'newsecurepass12');
    await user.click(screen.getByRole('button', { name: 'Reset' }));

    // Verify PUT was sent with correct body
    await waitFor(() => {
      const putCall = mockFetch.mock.calls.find(
        ([url, opts]) =>
          String(url).startsWith('/api/v1/admin/users/') &&
          String(url).endsWith('/password') &&
          (opts as RequestInit)?.method === 'PUT',
      );
      expect(putCall).toBeDefined();
      if (putCall) {
        const body = JSON.parse((putCall[1] as RequestInit).body as string);
        expect(body.password).toBe('newsecurepass12');
      }
    });
  });

  // ── Reset Password: password mismatch ───────────────────────────────

  it('prevents reset submission when passwords do not match', async () => {
    const user = userEvent.setup();
    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/admin/users' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleUsers));
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    await renderAdminPage();
    await waitFor(() => {
      expect(screen.getByText('dev@flux.dev')).toBeInTheDocument();
    });

    const resetButtons = screen.getAllByRole('button', { name: /reset password/i });
    await user.click(resetButtons[1]!);

    await waitFor(() => {
      expect(screen.getByLabelText(/^new password$/i)).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText(/^new password$/i), 'newsecurepass12');
    await user.type(screen.getByLabelText(/^confirm/i), 'differentpass12');
    await user.click(screen.getByRole('button', { name: /^reset$/i }));

    // Should show mismatch error, not send PUT
    await waitFor(() => {
      expect(screen.getByText(/passwords do not match/i)).toBeInTheDocument();
    });
  });

  // ── Reset Password: API error ───────────────────────────────────────

  it('shows error when reset password API call fails', async () => {
    const user = userEvent.setup();
    mockFetch.mockImplementation((url: string, options?: RequestInit) => {
      const method = options?.method || 'GET';
      if (url === '/api/v1/admin/users' && method === 'GET') {
        return Promise.resolve(jsonResponse(sampleUsers));
      }
      if (
        url.startsWith('/api/v1/admin/users/') &&
        url.endsWith('/password') &&
        method === 'PUT'
      ) {
        return Promise.resolve(jsonErrorResponse(500, 'Internal Server Error'));
      }
      return Promise.reject(new Error(`Unexpected fetch: ${method} ${url}`));
    });

    await renderAdminPage();
    await waitFor(() => {
      expect(screen.getByText('dev@flux.dev')).toBeInTheDocument();
    });

    const resetButtons = screen.getAllByRole('button', { name: /reset password/i });
    await user.click(resetButtons[1]!);

    await waitFor(() => {
      expect(screen.getByLabelText(/^new password$/i)).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText(/^new password$/i), 'newsecurepass12');
    await user.type(screen.getByLabelText(/^confirm/i), 'newsecurepass12');
    await user.click(screen.getByRole('button', { name: /^reset$/i }));

    await waitFor(() => {
      expect(screen.getByText(/internal server error/i)).toBeInTheDocument();
    });
  });
});
