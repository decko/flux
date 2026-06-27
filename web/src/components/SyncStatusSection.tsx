import { useQuery } from '@tanstack/react-query';

interface SyncStatus {
  last_sync_at: string | null;
  last_sync_error: string;
  tickets_synced: number;
  prs_synced: number;
  webhooks_healthy: boolean;
}

/**
 * Fetches the current sync status from the API.
 * Reads the JWT token from localStorage for authorization.
 */
async function fetchSyncStatus(): Promise<SyncStatus> {
  const token = localStorage.getItem('flux_token');
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const res = await fetch('/api/v1/sync/status', { headers });
  if (!res.ok) throw new Error('Failed to fetch sync status');
  return res.json() as Promise<SyncStatus>;
}

/**
 * Converts an ISO date string into a human-readable relative time.
 * Returns "never" for null/undefined input.
 */
function formatRelativeTime(iso: string | null): string {
  if (!iso) return 'never';
  const now = new Date();
  const d = new Date(iso);
  const diffMs = now.getTime() - d.getTime();
  const diffSeconds = Math.floor(diffMs / 1000);

  if (diffSeconds < 60) return 'Just now';
  const diffMinutes = Math.floor(diffSeconds / 60);
  if (diffMinutes < 60) return `${diffMinutes}m ago`;
  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) return `${diffHours}h ago`;
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays < 30) return `${diffDays}d ago`;
  return d.toLocaleDateString();
}

/**
 * Returns a Tailwind text color class based on how recent the sync was.
 * - Green: less than 15 minutes ago
 * - Yellow: 15-60 minutes ago
 * - Red: more than 60 minutes ago, or never synced
 */
function lastSyncColor(iso: string | null): string {
  if (!iso) return 'text-red-600';
  const now = new Date();
  const d = new Date(iso);
  const diffMs = now.getTime() - d.getTime();
  const diffMinutes = Math.floor(diffMs / 60_000);

  if (diffMinutes < 15) return 'text-green-600';
  if (diffMinutes < 60) return 'text-yellow-600';
  return 'text-red-600';
}

/**
 * SyncStatusSection fetches sync status from GET /api/v1/sync/status
 * and displays a compact health bar with last sync time, webhook health,
 * and ticket/PR counts. Auto-refreshes every 30 seconds.
 */
export function SyncStatusSection() {
  const { data, isLoading, isError, error, refetch } = useQuery<SyncStatus>({
    queryKey: ['sync-status'],
    queryFn: fetchSyncStatus,
    refetchInterval: 30_000,
  });

  if (isLoading) {
    return (
      <div className="mt-4 rounded-lg border border-gray-200 bg-white p-3 text-sm">
        Loading sync status...
      </div>
    );
  }

  if (isError) {
    const message = error instanceof Error ? error.message : String(error);
    return (
      <div className="mt-4 rounded-lg border border-gray-200 bg-white p-3 text-sm">
        <div className="flex items-center justify-between">
          <span className="text-red-600">Sync status: {message}</span>
          <button
            type="button"
            onClick={() => refetch()}
            className="rounded-md bg-blue-50 px-3 py-1 text-sm font-medium text-blue-700 hover:bg-blue-100"
          >
            Retry
          </button>
        </div>
      </div>
    );
  }

  const { last_sync_at, webhooks_healthy, tickets_synced, prs_synced } = data!;

  return (
    <div className="mt-4 rounded-lg border border-gray-200 bg-white p-3 text-sm">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-gray-600">
        <span>
          Last sync:{' '}
          <span className={`font-medium ${lastSyncColor(last_sync_at)}`}>
            {formatRelativeTime(last_sync_at)}
          </span>
        </span>
        <span aria-hidden="true">·</span>
        <span>
          Webhooks{' '}
          <span
            className={
              webhooks_healthy
                ? 'text-green-500'
                : 'text-red-500'
            }
          >
            ●
          </span>{' '}
          {webhooks_healthy ? 'healthy' : 'unhealthy'}
        </span>
        <span aria-hidden="true">·</span>
        <span>
          {tickets_synced} tickets
        </span>
        <span aria-hidden="true">·</span>
        <span>
          {prs_synced} PRs
        </span>
      </div>
    </div>
  );
}
