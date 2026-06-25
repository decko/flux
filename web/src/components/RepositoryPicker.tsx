import { useQuery } from '@tanstack/react-query';
import { fetchInstallationRepos, type GitHubInstallationRepo } from '@/api/github';

export interface RepositoryPickerProps {
  installationId: number;
  onSelect: (repo: GitHubInstallationRepo) => void;
}

/**
 * RepositoryPicker fetches the list of repositories for a given
 * GitHub App installation and renders them as clickable options.
 *
 * Used in the project creation wizard (Step 2) after the user
 * has selected an installation.
 */
export function RepositoryPicker({
  installationId,
  onSelect,
}: RepositoryPickerProps) {
  const query = useQuery<GitHubInstallationRepo[]>({
    queryKey: ['github-repos', installationId],
    queryFn: () => fetchInstallationRepos(installationId),
  });

  if (query.isPending) {
    return (
      <div className="text-sm text-gray-500" role="status" aria-label="loading">
        Loading repositories...
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
          : 'Failed to load repositories'}
      </div>
    );
  }

  if (query.data.length === 0) {
    return (
      <div
        role="status"
        className="rounded-lg border border-dashed border-gray-300 p-8 text-center text-gray-500"
      >
        No repositories found for this installation
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {query.data.map((repo) => (
        <button
          key={repo.id}
          type="button"
          onClick={() => onSelect(repo)}
          className="w-full rounded-md border border-gray-200 bg-white px-4 py-3 text-left text-sm hover:bg-gray-50"
        >
          <span className="font-medium text-gray-900">{repo.name}</span>
          {repo.private && (
            <span className="ml-2 text-xs text-gray-500">Private</span>
          )}
        </button>
      ))}
    </div>
  );
}
