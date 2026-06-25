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

/* ── Stub functions – will be implemented in GREEN phase ── */

export async function fetchInstallations(): Promise<GitHubInstallation[]> {
  throw new Error('not implemented');
}

export async function fetchInstallationRepos(
  _installationId: number,
): Promise<GitHubInstallationRepo[]> {
  void _installationId;
  throw new Error('not implemented');
}
