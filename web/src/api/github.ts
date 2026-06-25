// Types for the GitHub discovery API.

export interface GitHubInstallation {
  id: number;
  account: {
    login: string;
  };
  target_type: string;
  html_url: string;
}

export interface GitHubInstallationRepo {
  id: number;
  name: string;
  full_name: string;
  html_url: string;
  private: boolean;
}

/** Read JWT token from localStorage (set by login flow). */
function getToken(): string | null {
  try {
    return localStorage.getItem('flux_token');
  } catch {
    return null;
  }
}

/**
 * Fetches all GitHub App installations accessible by the authenticated user.
 * GET /api/v1/github/installations → GitHubInstallation[]
 */
export async function fetchInstallations(): Promise<GitHubInstallation[]> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch('/api/v1/github/installations', { headers });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(
      (body as Record<string, unknown>).error as string || res.statusText,
    );
  }
  return res.json() as Promise<GitHubInstallation[]>;
}

/**
 * Fetches repositories accessible through a specific GitHub App installation.
 * GET /api/v1/github/installations/:id/repositories → GitHubInstallationRepo[]
 */
export async function fetchInstallationRepos(
  installationId: number,
): Promise<GitHubInstallationRepo[]> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(
    `/api/v1/github/installations/${installationId}/repositories`,
    { headers },
  );
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(
      (body as Record<string, unknown>).error as string || res.statusText,
    );
  }
  return res.json() as Promise<GitHubInstallationRepo[]>;
}
