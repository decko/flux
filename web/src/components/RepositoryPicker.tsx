import { useQuery } from '@tanstack/react-query';
import { fetchInstallationRepos } from '@/api/github';
import type { GitHubInstallationRepo } from '@/api/github';

export interface RepositoryPickerProps {
  installationId: string;
  onSelect: (repo: GitHubInstallationRepo) => void;
}

/**
 * RepositoryPicker fetches repositories for a given GitHub App installation
 * using TanStack Query and renders them as a selectable, filterable list.
 * Supports loading, error, empty, and success states.
 */
export function RepositoryPicker({
  installationId,
  onSelect,
}: RepositoryPickerProps) {
  const { data, isPending, isError, error } = useQuery({
    queryKey: ['github-repos', installationId],
    queryFn: () => fetchInstallationRepos(Number(installationId)),
  });

  // --- Loading state ---
  if (isPending) {
    return (
      <div
        role="status"
        aria-label="Loading repositories"
        className="space-y-3"
      >
        {[1, 2, 3].map((i) => (
          <div
            key={i}
            className="animate-pulse rounded-lg border border-gray-200 bg-white p-4"
          >
            <div className="h-4 w-1/3 rounded bg-gray-200" />
            <div className="mt-2 h-3 w-1/2 rounded bg-gray-200" />
          </div>
        ))}
      </div>
    );
  }

  // --- Error state ---
  if (isError) {
    const message =
      error instanceof Error ? error.message : String(error);
    return (
      <div
        role="alert"
        className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800"
      >
        {message}
      </div>
    );
  }

  // --- Empty state ---
  if (data.length === 0) {
    return (
      <div
        role="status"
        className="rounded-lg border border-dashed border-gray-300 p-8 text-center text-gray-500"
      >
        No repositories found
      </div>
    );
  }

  // --- Success state ---
  return (
    <div className="space-y-2">
      {data.map((repo) => (
        <button
          key={repo.id}
          type="button"
          onClick={() => onSelect(repo)}
          className="w-full cursor-pointer rounded-lg border border-gray-200 bg-white p-4 text-left shadow-sm transition-colors hover:border-blue-400 hover:shadow-md"
        >
          <div className="flex items-center justify-between">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-gray-900">
                  {repo.name}
                </span>
                {repo.private && (
                  <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                    Private
                  </span>
                )}
              </div>
              <p className="truncate text-sm text-gray-500">
                {repo.full_name}
              </p>
            </div>
          </div>
        </button>
      ))}
    </div>
  );
}
