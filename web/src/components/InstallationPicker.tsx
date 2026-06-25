import type { GitHubInstallation } from '@/api/github';

export interface InstallationPickerProps {
  onSelect: (installation: GitHubInstallation) => void;
}

/** Stub — will be implemented in GREEN phase. */
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function InstallationPicker(_props: InstallationPickerProps) {
  return null;
}
