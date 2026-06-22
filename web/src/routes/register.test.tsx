import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  createRouter,
  createRootRoute,
  createRoute,
  RouterProvider,
  createMemoryHistory,
  Outlet,
} from '@tanstack/react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { AuthProvider } from '../auth/AuthContext';
import { RegisterPage } from './register';

// ---- Helpers ----

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

// ---- Test wrapper with router ----

function createTestRouter(initialPath: string) {
  const rootRoute = createRootRoute({
    component: () => <Outlet />,
  });

  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    component: () => <div data-testid="home-page">Home</div>,
  });

  const testRegisterRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/register',
    component: RegisterPage,
  });

  const routeTree = rootRoute.addChildren([indexRoute, testRegisterRoute]);
  const memoryHistory = createMemoryHistory({ initialEntries: [initialPath] });
  return createRouter({ routeTree, history: memoryHistory });
}

async function renderRegisterPage(initialPath = '/register') {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
    },
  });

  const router = createTestRouter(initialPath);
  await router.load();

  return render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <RouterProvider router={router} />
      </AuthProvider>
    </QueryClientProvider>,
  );
}

// ---- Tests ----

describe('RegisterPage', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // --- Rendering ---

  it('renders the register form with email and password fields', async () => {
    await renderRegisterPage();

    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /create account/i }),
    ).toBeInTheDocument();
  });

  it('renders a heading with "Create Account"', async () => {
    await renderRegisterPage();

    expect(
      screen.getByRole('heading', { name: /create account/i }),
    ).toBeInTheDocument();
  });

  // --- Loading state ---

  it('shows a loading indicator on the button while submitting', async () => {
    // Register call hangs indefinitely
    mockFetch.mockReturnValue(new Promise(() => {}));

    const user = userEvent.setup();
    await renderRegisterPage();

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'pwd123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    expect(
      screen.getByRole('button', { name: /creating account/i }),
    ).toBeInTheDocument();
  });

  // --- Success state ---

  it('sends a POST request to /api/v1/auth/register with email and password', async () => {
    // Register succeeds
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ id: 'user-1', email: 'new@example.com' }, 201),
    );
    // Then login is called
    mockFetch.mockResolvedValueOnce(jsonResponse({ token: 'jwt-token' }));

    const user = userEvent.setup();
    await renderRegisterPage();

    await user.type(screen.getByLabelText(/email/i), 'new@example.com');
    await user.type(screen.getByLabelText(/password/i), 'strong-pwd');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/auth/register',
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            email: 'new@example.com',
            password: 'strong-pwd',
          }),
        }),
      );
    });
  });

  it('auto-logins after successful registration', async () => {
    // Register succeeds
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ id: 'u-1', email: 'a@b.com' }, 201),
    );
    // Login is called automatically
    mockFetch.mockResolvedValueOnce(jsonResponse({ token: 'auto-jwt' }));

    const user = userEvent.setup();
    await renderRegisterPage();

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'pwd123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    // Should call login with same credentials
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/auth/login',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ email: 'a@b.com', password: 'pwd123' }),
        }),
      );
    });
  });

  it('stores the token in localStorage on successful registration flow', async () => {
    // Register succeeds
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ id: 'u-1', email: 'a@b.com' }, 201),
    );
    // Login succeeds
    mockFetch.mockResolvedValueOnce(jsonResponse({ token: 'reg-jwt' }));

    const user = userEvent.setup();
    await renderRegisterPage();

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'pwd123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() => {
      expect(localStorage.getItem('flux_token')).toBe('reg-jwt');
    });
  });

  it('redirects to the home page after successful registration', async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ id: 'u-1', email: 'a@b.com' }, 201),
    );
    mockFetch.mockResolvedValueOnce(jsonResponse({ token: 'jwt' }));

    const user = userEvent.setup();
    await renderRegisterPage();

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'pwd123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() => {
      expect(screen.getByTestId('home-page')).toBeInTheDocument();
    });
  });

  // --- Error state ---

  it('shows an error banner on 409 (duplicate email)', async () => {
    const errorBody = JSON.stringify({ error: 'email already exists' });
    mockFetch.mockResolvedValueOnce(
      new Response(errorBody, {
        status: 409,
        statusText: 'Conflict',
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    const user = userEvent.setup();
    await renderRegisterPage();

    await user.type(screen.getByLabelText(/email/i), 'exists@b.com');
    await user.type(screen.getByLabelText(/password/i), 'pwd123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/email already exists/i)).toBeInTheDocument();
    });
  });

  it('shows an error banner on 400 validation error', async () => {
    const errorBody = JSON.stringify({
      error: 'password must be at least 8 characters',
    });
    mockFetch.mockResolvedValueOnce(
      new Response(errorBody, {
        status: 400,
        statusText: 'Bad Request',
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    const user = userEvent.setup();
    await renderRegisterPage();

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'short');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(
        screen.getByText(/password must be at least 8 characters/i),
      ).toBeInTheDocument();
    });
  });

  it('shows an error banner when auto-login fails after registration', async () => {
    // Register succeeds
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ id: 'u-1', email: 'a@b.com' }, 201),
    );
    // Login fails
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: 'invalid credentials' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    const user = userEvent.setup();
    await renderRegisterPage();

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'pwd123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(
        screen.getByText(/registration succeeded.*login failed/i),
      ).toBeInTheDocument();
    });
  });

  it('shows an error banner on network failure', async () => {
    mockFetch.mockRejectedValue(new Error('Network Error'));

    const user = userEvent.setup();
    await renderRegisterPage();

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'pwd123');
    await user.click(screen.getByRole('button', { name: /create account/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/network error/i)).toBeInTheDocument();
    });
  });
});
