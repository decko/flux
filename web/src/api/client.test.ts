import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { apiGet, setRefreshCallback } from './client';

function mockFetchOnce(
  status: number,
  body: unknown,
  ok?: boolean,
): void {
  const isOk = ok ?? (status >= 200 && status < 300);
  const fetcher = vi.fn().mockResolvedValueOnce({
    ok: isOk,
    status,
    statusText: status === 401 ? 'Unauthorized' : 'OK',
    text: () => Promise.resolve(typeof body === 'string' ? body : JSON.stringify(body)),
    json: () => Promise.resolve(body),
    headers: new Headers({ 'content-type': 'application/json' }),
  });
  vi.stubGlobal('fetch', fetcher);
}

function mockFetchSequence(
  responses: Array<{ status: number; body: unknown; ok?: boolean }>,
): void {
  const fetcher = vi.fn();
  for (const r of responses) {
    const isOk = r.ok ?? (r.status >= 200 && r.status < 300);
    fetcher.mockResolvedValueOnce({
      ok: isOk,
      status: r.status,
      statusText: r.status === 401 ? 'Unauthorized' : 'OK',
      text: () =>
        Promise.resolve(
          typeof r.body === 'string' ? r.body : JSON.stringify(r.body),
        ),
      json: () => Promise.resolve(r.body),
      headers: new Headers({ 'content-type': 'application/json' }),
    });
  }
  vi.stubGlobal('fetch', fetcher);
}

describe('api client', () => {
  let tokenCallback: ((token: string | null) => void) | null = null;

  beforeEach(() => {
    localStorage.clear();
    vi.unstubAllGlobals();
    tokenCallback = null;
    setRefreshCallback((token) => {
      tokenCallback = token;
    });
  });

  afterEach(() => {
    setRefreshCallback(null);
  });

  describe('Authorization header', () => {
    it('adds Authorization header when token exists in localStorage', async () => {
      localStorage.setItem('flux_token', 'my-token');
      mockFetchOnce(200, { data: 'ok' });

      await apiGet('/test');

      const fetchMock = vi.mocked(globalThis.fetch);
      expect(fetchMock).toHaveBeenCalledTimes(1);
      const [, options] = fetchMock.mock.calls[0] as [string, RequestInit];
      const headers = options.headers as Record<string, string>;
      expect(headers['Authorization']).toBe('Bearer my-token');
    });

    it('does not add Authorization header when no token', async () => {
      mockFetchOnce(200, { data: 'ok' });

      await apiGet('/test');

      const fetchMock = vi.mocked(globalThis.fetch);
      const [, options] = fetchMock.mock.calls[0] as [string, RequestInit];
      const headers = options.headers as Record<string, string>;
      expect(headers['Authorization']).toBeUndefined();
    });
  });

  describe('401 interceptor with token refresh', () => {
    it('on 401, attempts refresh, retries original request on success', async () => {
      localStorage.setItem('flux_token', 'expired-token');

      // First call: 401, Second call: refresh OK, Third call: retry OK
      mockFetchSequence([
        { status: 401, body: 'Unauthorized', ok: false },
        { status: 200, body: { token: 'new-token' } },
        { status: 200, body: { data: 'retried-ok' } },
      ]);

      const result = await apiGet<{ data: string }>('/protected');

      expect(result).toEqual({ data: 'retried-ok' });
      expect(localStorage.getItem('flux_token')).toBe('new-token');
      expect(tokenCallback).toBe('new-token');

      const fetchMock = vi.mocked(globalThis.fetch);
      // Call 1: original request -> 401
      // Call 2: refresh POST
      // Call 3: retried request
      expect(fetchMock).toHaveBeenCalledTimes(3);

      // Verify refresh call
      const refreshCall = fetchMock.mock.calls[1] as [string, RequestInit];
      expect(refreshCall[0]).toBe('/api/v1/auth/refresh');
      expect(refreshCall[1]?.method).toBe('POST');
    });

    it('on refresh failure, clears token and does not retry', async () => {
      localStorage.setItem('flux_token', 'expired-token');

      mockFetchSequence([
        { status: 401, body: 'Unauthorized', ok: false },
        { status: 401, body: 'Unauthorized', ok: false },
      ]);

      await expect(apiGet('/protected')).rejects.toThrow();

      expect(localStorage.getItem('flux_token')).toBeNull();
      expect(tokenCallback).toBeNull();

      const fetchMock = vi.mocked(globalThis.fetch);
      expect(fetchMock).toHaveBeenCalledTimes(2);
    });

    it('propagates non-401 errors without attempting refresh', async () => {
      mockFetchOnce(500, 'Server Error', false);

      await expect(apiGet('/fail')).rejects.toThrow();

      const fetchMock = vi.mocked(globalThis.fetch);
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it('does not retry more than once on persistent 401', async () => {
      localStorage.setItem('flux_token', 'expired-token');

      // Refresh works, but retry still gets 401
      mockFetchSequence([
        { status: 401, body: 'Unauthorized', ok: false },
        { status: 200, body: { token: 'new-token' } },
        { status: 401, body: 'Unauthorized', ok: false },
      ]);

      await expect(apiGet('/protected')).rejects.toThrow();

      const fetchMock = vi.mocked(globalThis.fetch);
      expect(fetchMock).toHaveBeenCalledTimes(3);
    });

    it('handles network error during refresh gracefully', async () => {
      localStorage.setItem('flux_token', 'expired-token');

      const fetcher = vi
        .fn()
        .mockResolvedValueOnce({
          ok: false,
          status: 401,
          statusText: 'Unauthorized',
          text: () => Promise.resolve('Unauthorized'),
          json: () => Promise.resolve({}),
          headers: new Headers(),
        })
        .mockRejectedValueOnce(new Error('Network error'))
        .mockResolvedValueOnce({
          ok: true,
          status: 200,
          text: () => Promise.resolve('{}'),
          json: () => Promise.resolve({}),
          headers: new Headers(),
        });
      vi.stubGlobal('fetch', fetcher);

      // Should fail because refresh failed
      await expect(apiGet('/protected')).rejects.toThrow();
      expect(localStorage.getItem('flux_token')).toBeNull();
    });
  });

  describe('parallel refresh prevention', () => {
    it('prevents parallel refresh requests', async () => {
      localStorage.setItem('flux_token', 'expired-token');

      let refreshCount = 0;
      let hasRefreshed = false;

      const fetcher = vi.fn().mockImplementation(
        (url: string) => {
          if (url === '/api/v1/auth/refresh') {
            refreshCount++;
            hasRefreshed = true;
            return Promise.resolve({
              ok: true,
              status: 200,
              statusText: 'OK',
              text: () => Promise.resolve(JSON.stringify({ token: 'refreshed' })),
              json: () => Promise.resolve({ token: 'refreshed' }),
              headers: new Headers({ 'content-type': 'application/json' }),
            });
          }

          // After refresh, requests succeed; before refresh, they 401
          if (hasRefreshed) {
            return Promise.resolve({
              ok: true,
              status: 200,
              statusText: 'OK',
              text: () => Promise.resolve(JSON.stringify({ ok: true })),
              json: () => Promise.resolve({ ok: true }),
              headers: new Headers({ 'content-type': 'application/json' }),
            });
          }

          return Promise.resolve({
            ok: false,
            status: 401,
            statusText: 'Unauthorized',
            text: () => Promise.resolve('Unauthorized'),
            json: () => Promise.resolve({}),
            headers: new Headers(),
          });
        },
      );
      vi.stubGlobal('fetch', fetcher);

      // Fire two requests in parallel
      const [r1, r2] = await Promise.allSettled([
        apiGet('/a'),
        apiGet('/b'),
      ]);

      // Both should eventually succeed after the single refresh
      expect(r1.status).toBe('fulfilled');
      expect(r2.status).toBe('fulfilled');
      expect(refreshCount).toBe(1);
    });
  });
});
