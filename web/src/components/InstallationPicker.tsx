import { useQuery } from '@tanstack/react-query';
import { fetchInstallations } from '@/api/github';
import type { GitHubInstallation } from '@/api/github';

export interface InstallationPickerProps {
  onSelect: (installation: GitHubInstallation) => void;
}

/**
 * InstallationPicker fetches GitHub App installations using TanStack Query
 * and renders them as selectable cards. Supports loading, error, empty, and
 * success states.
 */
export function InstallationPicker({ onSelect }: InstallationPickerProps) {
  const { data, isPending, isError, error } = useQuery({
    queryKey: ['github-installations'],
    queryFn: fetchInstallations,
  });

  // --- Loading state ---
  if (isPending) {
    return (
      <div
        role="status"
        aria-label="Loading installations"
        className="space-y-3"
      >
        {[1, 2].map((i) => (
          <div
            key={i}
            className="animate-pulse rounded-lg border border-gray-200 bg-white p-4"
          >
            <div className="h-4 w-1/3 rounded bg-gray-200" />
            <div className="mt-2 h-3 w-1/4 rounded bg-gray-200" />
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
        No installations found
      </div>
    );
  }

  // --- Success state ---
  return (
    <div className="grid gap-3">
      {data.map((installation) => (
        <button
          key={installation.id}
          type="button"
          onClick={() => onSelect(installation)}
          className="w-full cursor-pointer rounded-lg border border-gray-200 bg-white p-4 text-left shadow-sm transition-colors hover:border-blue-400 hover:shadow-md"
        >
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-gray-900">
              {installation.account.login}
            </span>
            <span
              className={
                installation.target_type === 'Organization'
                  ? 'inline-flex items-center rounded-full bg-blue-50 px-2.5 py-0.5 text-xs font-medium text-blue-700'
                  : 'inline-flex items-center rounded-full bg-green-50 px-2.5 py-0.5 text-xs font-medium text-green-700'
              }
            >
              {installation.target_type}
            </span>
          </div>
        </button>
      ))}
    </div>
  );
}
