import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fetchInstallations, fetchInstallationRepos } from './github';
import type { GitHubInstallation, GitHubInstallationRepo } from './github';

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

const sampleInstallations: GitHubInstallation[] = [
  {
    id: 101,
    account: { login: 'flux-org' },
    target_type: 'Organization',
    html_url: 'https://github.com/organizations/flux-org',
  },
  {
    id: 202,
    account: { login: 'decko' },
    target_type: 'User',
    html_url: 'https://github.com/decko',
  },
];

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

describe('fetchInstallations', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('returns installations on success', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleInstallations));

    const result = await fetchInstallations();

    expect(result).toEqual(sampleInstallations);
  });

  it('throws on 503 (GitHub App not configured)', async () => {
    mockFetch.mockResolvedValue(
      jsonErrorResponse(503, 'GitHub App not configured'),
    );

    await expect(fetchInstallations()).rejects.toThrow(
      'GitHub App not configured',
    );
  });

  it('throws on network error', async () => {
    mockFetch.mockRejectedValue(new Error('Network Error'));

    await expect(fetchInstallations()).rejects.toThrow('Network Error');
  });
});

describe('fetchInstallationRepos', () => {
  let mockFetch: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockFetch = vi.fn();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('returns repos on success', async () => {
    mockFetch.mockResolvedValue(jsonResponse(sampleRepos));

    const result = await fetchInstallationRepos(101);

    expect(result).toEqual(sampleRepos);
  });

  it('throws on 503 (GitHub App not configured)', async () => {
    mockFetch.mockResolvedValue(
      jsonErrorResponse(503, 'GitHub App not configured'),
    );

    await expect(fetchInstallationRepos(202)).rejects.toThrow(
      'GitHub App not configured',
    );
  });

  it('throws on network error', async () => {
    mockFetch.mockRejectedValue(new Error('Network Error'));

    await expect(fetchInstallationRepos(1)).rejects.toThrow('Network Error');
  });
});
