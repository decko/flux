import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { AdapterCard } from './AdapterCard';

interface AdapterInfo {
  type: string;
  name: string;
  health: 'healthy' | 'unhealthy' | 'unknown';
}

interface SyncStatus {
  lastSyncAt: string | null;
  lastSyncError: string;
  ticketsSynced: number;
  prsSynced: number;
}

/**
 * Fetches the list of configured adapters.
 * GET /api/v1/adapters → AdapterInfo[]
 */
async function fetchAdapters(): Promise<AdapterInfo[]> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch('/api/v1/adapters', { headers });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || res.statusText);
  }
  return res.json();
}

/**
 * Fetches the global sync status.
 * GET /api/v1/sync/status → SyncStatus
 */
async function fetchSyncStatus(): Promise<SyncStatus> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch('/api/v1/sync/status', { headers });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || res.statusText);
  }
  return res.json();
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
 * AdapterList fetches configured adapters and global sync status,
 * displaying a list of AdapterCard components and a global sync control.
 */
export function AdapterList() {
  const queryClient = useQueryClient();

  const adaptersQuery = useQuery<AdapterInfo[]>({
    queryKey: ['adapters'],
    queryFn: fetchAdapters,
  });

  const syncStatusQuery = useQuery<SyncStatus>({
    queryKey: ['sync-status'],
    queryFn: fetchSyncStatus,
  });

  const syncMutation = useMutation({
    mutationFn: async () => {
      const token = getToken();
      const headers: Record<string, string> = { 'Content-Type': 'application/json' };
      if (token) headers['Authorization'] = `Bearer ${token}`;

      const res = await fetch('/api/v1/sync/trigger', {
        method: 'POST',
        headers,
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || res.statusText);
      }
    },
    onSuccess: () => {
      // Invalidate both queries so the UI refreshes after sync.
      queryClient.invalidateQueries({ queryKey: ['sync-status'] });
      queryClient.invalidateQueries({ queryKey: ['adapters'] });
    },
  });

  // --- Loading state ---
  if (adaptersQuery.isPending || syncStatusQuery.isPending) {
    return <LoadingSkeleton />;
  }

  // --- Error state ---
  if (adaptersQuery.isError) {
    return (
      <ErrorBanner
        message={extractErrorMessage(adaptersQuery.error)}
        onRetry={() => adaptersQuery.refetch()}
      />
    );
  }

  if (syncStatusQuery.isError) {
    return (
      <ErrorBanner
        message={extractErrorMessage(syncStatusQuery.error)}
        onRetry={() => syncStatusQuery.refetch()}
      />
    );
  }

  const adapters = adaptersQuery.data;
  const syncStatus = syncStatusQuery.data;

  // --- Empty state ---
  if (adapters.length === 0) {
    return (
      <div
        role="status"
        className="rounded-lg border border-dashed border-gray-300 p-8 text-center text-gray-500"
      >
        No adapters configured
      </div>
    );
  }

  // --- Success state ---
  return (
    <div className="space-y-4">
      {/* Global sync controls */}
      <div className="flex items-center justify-between rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
        <div className="flex flex-wrap gap-4 text-sm text-gray-500">
          <span>
            Tickets synced:{' '}
            <span className="font-medium text-gray-700">
              {syncStatus.ticketsSynced}
            </span>
          </span>
          <span>
            PRs synced:{' '}
            <span className="font-medium text-gray-700">
              {syncStatus.prsSynced}
            </span>
          </span>
          <span>
            Last sync:{' '}
            <span className="font-medium text-gray-700">
              {formatLastSync(syncStatus.lastSyncAt)}
            </span>
          </span>
        </div>
        <button
          type="button"
          disabled={syncMutation.isPending}
          onClick={() => syncMutation.mutate()}
          className="inline-flex items-center gap-1.5 rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {syncMutation.isPending && (
            <span
              className="inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-white border-t-transparent"
              aria-label="syncing"
            />
          )}
          Sync Now
        </button>
      </div>

      {/* Sync error banner (global) */}
      {syncStatus.lastSyncError && (
        <div
          role="alert"
          className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800"
        >
          {syncStatus.lastSyncError}
        </div>
      )}

      {/* Adapter cards */}
      {adapters.map((adapter) => (
        <AdapterCard
          key={adapter.type}
          type={adapter.type}
          name={adapter.name}
          health={adapter.health}
        />
      ))}
    </div>
  );
}

// --- Sub-components ---

function LoadingSkeleton() {
  return (
    <div className="space-y-4" role="status" aria-label="loading">
      {[1, 2, 3].map((i) => (
        <div
          key={i}
          className="animate-pulse rounded-lg border border-gray-200 bg-white p-4"
        >
          <div className="h-4 w-1/3 rounded bg-gray-200" />
          <div className="mt-2 h-4 w-1/2 rounded bg-gray-200" />
        </div>
      ))}
    </div>
  );
}

interface ErrorBannerProps {
  message: string;
  onRetry: () => void;
}

function ErrorBanner({ message, onRetry }: ErrorBannerProps) {
  return (
    <div
      role="alert"
      className="rounded-lg border border-red-200 bg-red-50 p-4"
    >
      <div className="flex items-center justify-between">
        <p className="text-sm text-red-800">{message}</p>
        <button
          type="button"
          onClick={onRetry}
          className="rounded-md bg-red-100 px-3 py-1.5 text-sm font-medium text-red-700 hover:bg-red-200"
        >
          Retry
        </button>
      </div>
    </div>
  );
}

function extractErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

function formatLastSync(iso: string | null | undefined): string {
  if (!iso) return 'Never';
  const now = new Date();
  const d = new Date(iso);
  const diffMs = now.getTime() - d.getTime();
  const diffHours = Math.floor(diffMs / 3_600_000);
  const diffDays = Math.floor(diffMs / 86_400_000);

  if (diffHours < 1) return 'Less than an hour ago';
  if (diffDays < 1) return 'Less than a day ago';
  if (diffDays === 1) return 'Yesterday';
  if (diffDays < 30) return `${diffDays} days ago`;
  return d.toLocaleDateString();
}
