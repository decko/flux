import { useQuery } from '@tanstack/react-query';
import { fetchInstallations, type GitHubInstallation } from '@/api/github';

export interface InstallationPickerProps {
  onSelect: (installation: GitHubInstallation) => void;
}

/**
 * InstallationPicker fetches the list of GitHub App installations
 * and renders them as clickable options.
 *
 * Used in the project creation wizard (Step 1) to let the user
 * choose which GitHub installation to use for repository discovery.
 */
export function InstallationPicker({ onSelect }: InstallationPickerProps) {
  const query = useQuery<GitHubInstallation[]>({
    queryKey: ['github-installations'],
    queryFn: fetchInstallations,
  });

  if (query.isPending) {
    return (
      <div className="text-sm text-gray-500" role="status" aria-label="loading">
        Loading installations...
      </div>
    );
  }

  if (query.isError) {
    return (
      <div
        role="alert"
        className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800"
      >
        {query.error instanceof Error
          ? query.error.message
          : 'Failed to load installations'}
      </div>
    );
  }

  if (query.data.length === 0) {
    return (
      <div
        role="status"
        className="rounded-lg border border-dashed border-gray-300 p-8 text-center text-gray-500"
      >
        No installations found
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {query.data.map((inst) => (
        <button
          key={inst.id}
          type="button"
          onClick={() => onSelect(inst)}
          className="w-full rounded-md border border-gray-200 bg-white px-4 py-3 text-left text-sm hover:bg-gray-50"
        >
          <span className="font-medium text-gray-900">
            {inst.account.login}
          </span>
          <span className="ml-2 text-gray-500">({inst.target_type})</span>
        </button>
      ))}
    </div>
  );
}
