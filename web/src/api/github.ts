/**
 * GitHub API types for the frontend.
 *
 * These types correspond to the GitHub App installation and repository
 * resources returned by the backend proxy endpoints at /api/v1/github/*.
 */

export interface GitHubInstallation {
  id: number;
  account: { login: string };
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

/**
 * Fetches all GitHub App installations accessible to the authenticated user.
 * GET /api/v1/github/installations → GitHubInstallation[]
 */
export async function fetchInstallations(): Promise<GitHubInstallation[]> {
  const token = localStorage.getItem('flux_token');
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch('/api/v1/github/installations', { headers });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as Record<string, unknown>;
    throw new Error((body.error as string) || res.statusText);
  }
  return res.json() as Promise<GitHubInstallation[]>;
}

/**
 * Fetches repositories accessible through a specific GitHub App installation.
 * GET /api/v1/github/installations/{id}/repositories → GitHubInstallationRepo[]
 */
export async function fetchInstallationRepos(
  installationId: number,
): Promise<GitHubInstallationRepo[]> {
  const token = localStorage.getItem('flux_token');
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(
    `/api/v1/github/installations/${installationId}/repositories`,
    { headers },
  );
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as Record<string, unknown>;
    throw new Error((body.error as string) || res.statusText);
  }
  return res.json() as Promise<GitHubInstallationRepo[]>;
}
