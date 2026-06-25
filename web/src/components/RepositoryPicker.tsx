import type { GitHubInstallationRepo } from '@/api/github';

export interface RepositoryPickerProps {
  installationId: string;
  onSelect: (repo: GitHubInstallationRepo) => void;
}

/** Stub — will be implemented in GREEN phase. */
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function RepositoryPicker(_props: RepositoryPickerProps) {
  return null;
}
