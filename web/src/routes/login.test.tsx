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
import { LoginPage } from './login';

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

  const testLoginRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/login',
    component: LoginPage,
  });

  const routeTree = rootRoute.addChildren([indexRoute, testLoginRoute]);
  const memoryHistory = createMemoryHistory({ initialEntries: [initialPath] });
  return createRouter({ routeTree, history: memoryHistory });
}

async function renderLoginPage(initialPath = '/login') {
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

describe('LoginPage', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // --- Rendering ---

  it('renders the login form with email and password fields', async () => {
    await renderLoginPage();

    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /sign in/i }),
    ).toBeInTheDocument();
  });

  it('renders a heading with "Sign In"', async () => {
    await renderLoginPage();

    expect(
      screen.getByRole('heading', { name: /sign in/i }),
    ).toBeInTheDocument();
  });

  // --- Loading state ---

  it('disables the submit button while the request is in flight', async () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    const user = userEvent.setup();
    await renderLoginPage();

    const button = screen.getByRole('button', { name: /sign in/i });
    expect(button).not.toBeDisabled();

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'pwd');
    await user.click(button);

    expect(button).toBeDisabled();
  });

  it('shows a loading indicator on the button while submitting', async () => {
    mockFetch.mockReturnValue(new Promise(() => {}));

    await renderLoginPage();

    // The page auto-submits on load? No — button should only be disabled
    // when submitting. We need to trigger a submit first.

    // Actually the button text should say "Signing in..." when loading.
    // Let's trigger the form submit.
    const user = userEvent.setup();
    await user.type(screen.getByLabelText(/email/i), 'test@example.com');
    await user.type(screen.getByLabelText(/password/i), 'password123');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    expect(
      screen.getByRole('button', { name: /signing in/i }),
    ).toBeInTheDocument();
  });

  // --- Success state ---

  it('sends a POST request to /api/v1/auth/login with email and password', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ token: 'jwt-token' }));

    const user = userEvent.setup();
    await renderLoginPage();

    await user.type(screen.getByLabelText(/email/i), 'user@example.com');
    await user.type(screen.getByLabelText(/password/i), 'secret123');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/auth/login',
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            email: 'user@example.com',
            password: 'secret123',
          }),
        }),
      );
    });
  });

  it('stores the token in localStorage on successful login', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ token: 'test-jwt' }));

    const user = userEvent.setup();
    await renderLoginPage();

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'pwd');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(localStorage.getItem('flux_token')).toBe('test-jwt');
    });
  });

  it('redirects to the home page after successful login', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ token: 'jwt' }));

    const user = userEvent.setup();
    await renderLoginPage();

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'pwd');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByTestId('home-page')).toBeInTheDocument();
    });
  });

  // --- Error state ---

  it('shows an error banner on 401 response', async () => {
    const errorBody = JSON.stringify({ error: 'invalid credentials' });
    mockFetch.mockResolvedValue(
      new Response(errorBody, {
        status: 401,
        statusText: 'Unauthorized',
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    const user = userEvent.setup();
    await renderLoginPage();

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'wrong');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/invalid credentials/i)).toBeInTheDocument();
    });
  });

  it('shows an error banner on network failure', async () => {
    mockFetch.mockRejectedValue(new Error('Network Error'));

    const user = userEvent.setup();
    await renderLoginPage();

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'pwd');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText(/network error/i)).toBeInTheDocument();
    });
  });

  it('clears a previous error when re-submitting', async () => {
    // First attempt fails
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: 'invalid credentials' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    // Second attempt succeeds
    mockFetch.mockResolvedValueOnce(jsonResponse({ token: 'jwt' }));

    const user = userEvent.setup();
    await renderLoginPage();

    await user.type(screen.getByLabelText(/email/i), 'a@b.com');
    await user.type(screen.getByLabelText(/password/i), 'wrong');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    // Submit again with correct password
    await user.clear(screen.getByLabelText(/password/i));
    await user.type(screen.getByLabelText(/password/i), 'correct');
    await user.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.queryByRole('alert')).toBeNull();
      expect(screen.getByTestId('home-page')).toBeInTheDocument();
    });
  });
});
