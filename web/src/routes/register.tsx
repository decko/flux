import { useState } from 'react';
import { createRoute, useNavigate, useSearch, Link } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';
import { useAuth } from '../auth/AuthContext';

// --- Route ---

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/register',
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' ? search.redirect : undefined,
  }),
  component: RegisterPage,
});

// --- Helpers ---

interface AuthResponse {
  token?: string;
}

/**
 * Attempts to parse an API error response body and extract a user-facing
 * error message.
 */
function parseApiError(body: string): string | undefined {
  try {
    const parsed = JSON.parse(body) as Record<string, unknown>;
    const msg = parsed.error;
    if (typeof msg === 'string' && msg.length > 0) {
      return msg;
    }
  } catch {
    // Not JSON.
  }
  return undefined;
}

// --- Page component ---

/**
 * Register page with email and password form.
 *
 * On success the user is automatically signed in by calling the login
 * endpoint with the same credentials.
 *
 * States:
 * - idle: initial render
 * - loading: form submitted, awaiting response (button disabled + spinner)
 * - error: registration failure or auto-login failure (red banner)
 */
export function RegisterPage() {
  const navigate = useNavigate();
  const { redirect } = useSearch({ from: '/register' });
  const { login } = useAuth();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      // Step 1: Register
      const registerResponse = await fetch('/api/v1/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });

      if (!registerResponse.ok) {
        const body = await registerResponse
          .text()
          .catch(() => registerResponse.statusText);
        const message = parseApiError(body) ?? body;
        throw new Error(message);
      }

      // Step 2: Auto-login
      const loginResponse = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });

      if (!loginResponse.ok) {
        throw new Error(
          'Registration succeeded but login failed. Please try signing in.',
        );
      }

      const data = (await loginResponse.json()) as AuthResponse;
      if (!data.token) {
        throw new Error(
          'Registration succeeded but no token received. Please try signing in.',
        );
      }

      login(data.token);
      await navigate({ to: redirect || '/' });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : 'An unexpected error occurred';
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mx-auto mt-12 max-w-md">
      <h1 className="text-center text-2xl font-bold text-gray-900">
        Create Account
      </h1>

      <form
        onSubmit={handleSubmit}
        className="mt-8 rounded-lg border border-gray-200 bg-white p-6 shadow-sm"
        noValidate
      >
        {error && (
          <div
            role="alert"
            className="mb-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800"
          >
            {error}
          </div>
        )}

        <div className="space-y-4">
          <div>
            <label
              htmlFor="register-email"
              className="block text-sm font-medium text-gray-700"
            >
              Email
            </label>
            <input
              id="register-email"
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              placeholder="you@example.com"
            />
          </div>

          <div>
            <label
              htmlFor="register-password"
              className="block text-sm font-medium text-gray-700"
            >
              Password
            </label>
            <input
              id="register-password"
              type="password"
              autoComplete="new-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              placeholder="Create a password"
            />
          </div>
        </div>

        <button
          type="submit"
          disabled={loading}
          className="mt-6 flex w-full items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {loading ? (
            <>
              <svg
                className="-ml-1 mr-2 h-4 w-4 animate-spin text-white"
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
                aria-hidden="true"
              >
                <circle
                  className="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  strokeWidth="4"
                />
                <path
                  className="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"
                />
              </svg>
              Creating account...
            </>
          ) : (
            'Create Account'
          )}
        </button>

        <p className="mt-4 text-center text-sm text-gray-600">
          Already have an account?{' '}
          <Link
            to="/login" search={{ redirect: undefined }}
            className="font-medium text-blue-600 hover:text-blue-800"
          >
            Sign in
          </Link>
        </p>
      </form>
    </div>
  );
}
