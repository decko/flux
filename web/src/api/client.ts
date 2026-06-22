const API_BASE = '/api/v1';

type TokenRefreshCallback = (token: string | null) => void;

/** Registered by AuthProvider so the client can push refreshed tokens to React state. */
let refreshCallback: TokenRefreshCallback | null = null;

/**
 * Register a callback that fires whenever the token is refreshed or cleared
 * by the 401 interceptor. The AuthProvider calls this on mount.
 */
export function setRefreshCallback(cb: TokenRefreshCallback | null): void {
  refreshCallback = cb;
}

/** Retrieve the stored JWT token, if any. */
function getToken(): string | null {
  try {
    return localStorage.getItem('flux_token');
  } catch {
    return null;
  }
}

/** Persist a token to localStorage and notify the React layer. */
function saveToken(token: string): void {
  try {
    localStorage.setItem('flux_token', token);
  } catch {
    /* storage unavailable */
  }
  refreshCallback?.(token);
}

/** Remove the token from localStorage and notify the React layer. */
function clearToken(): void {
  try {
    localStorage.removeItem('flux_token');
  } catch {
    /* storage unavailable */
  }
  refreshCallback?.(null);
}

class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

/* ───── Refresh lock – prevents parallel refresh requests ───── */

let refreshPromise: Promise<string | null> | null = null;

/**
 * Attempt to refresh the JWT via the backend.
 * Uses a promise lock so concurrent 401s share a single refresh.
 * Returns the new token on success or null on failure.
 */
async function attemptRefresh(): Promise<string | null> {
  if (refreshPromise) return refreshPromise;

  refreshPromise = (async () => {
    try {
      const token = getToken();
      if (!token) return null;

      const response = await fetch(`${API_BASE}/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token }),
      });

      if (!response.ok) {
        clearToken();
        return null;
      }

      const data = (await response.json()) as { token: string };
      saveToken(data.token);
      return data.token;
    } catch {
      clearToken();
      return null;
    } finally {
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}

/* ───── Core request helper ───── */

async function executeRequest<T>(
  method: string,
  path: string,
  body?: unknown,
  isRetry = false,
): Promise<T> {
  const url = `${API_BASE}${path}`;
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  const token = getToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const options: RequestInit = { method, headers };

  if (body !== undefined) {
    options.body = JSON.stringify(body);
  }

  const response = await fetch(url, options);

  /* ── 401 interceptor ── */
  if (response.status === 401 && !isRetry) {
    const newToken = await attemptRefresh();
    if (newToken) {
      // Retry the original request with the fresh token
      return executeRequest<T>(method, path, body, true);
    }
    // Refresh failed – propagate the original 401
    const message = await response.text().catch(() => response.statusText);
    throw new ApiError(message, response.status);
  }

  if (!response.ok) {
    const message = await response.text().catch(() => response.statusText);
    throw new ApiError(message, response.status);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  return executeRequest<T>(method, path, body, false);
}

export async function apiGet<T>(path: string): Promise<T> {
  return request<T>('GET', path);
}

export async function apiPost<T>(
  path: string,
  body: unknown,
): Promise<T> {
  return request<T>('POST', path, body);
}

export async function apiPut<T>(
  path: string,
  body: unknown,
): Promise<T> {
  return request<T>('PUT', path, body);
}

export async function apiDelete(path: string): Promise<void> {
  return request<void>('DELETE', path);
}
