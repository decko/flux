import { useQuery } from '@tanstack/react-query';

export interface PipelineRunListProps {
  /** If provided, only runs for this ticket are fetched. Otherwise all runs are fetched. */
  ticketId?: string;
}

interface PipelineRunPhase {
  name: string;
  status: string;
  duration: number;
  output: string;
  error: string;
  started_at: string;
}

interface PipelineRun {
  id: string;
  project_id: string;
  ticket_id: string;
  orchestrator: string;
  pipeline: string;
  status: string;
  phases: PipelineRunPhase[];
  started_at: string;
  completed_at?: string;
  cost?: {
    total: number;
    currency: string;
    by_phase: Record<string, number>;
  };
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
 * Fetches pipeline runs, optionally filtered by ticket_id.
 * GET /api/v1/pipeline-runs[?ticket_id={ticketId}] → PipelineRun[]
 */
async function fetchPipelineRuns(ticketId?: string): Promise<PipelineRun[]> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  let url = '/api/v1/pipeline-runs';
  if (ticketId) {
    url += `?ticket_id=${encodeURIComponent(ticketId)}`;
  }

  const res = await fetch(url, { headers });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }

  return res.json() as Promise<PipelineRun[]>;
}

/**
 * PipelineRunList fetches pipeline runs (all or filtered by ticket) and displays them as cards.
 * Supports loading (skeleton), empty, error, and success states.
 */
export function PipelineRunList({ ticketId }: PipelineRunListProps) {
  const query = useQuery<PipelineRun[]>({
    queryKey: ticketId ? ['pipeline-runs', ticketId] : ['pipeline-runs'],
    queryFn: () => fetchPipelineRuns(ticketId),
  });

  // --- Loading state ---
  if (query.isPending) {
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

  // --- Error state ---
  if (query.isError) {
    const message = query.error instanceof Error ? query.error.message : String(query.error);
    return (
      <div
        role="alert"
        className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800"
      >
        {message}
      </div>
    );
  }

  const runs = query.data;

  // --- Empty state ---
  if (runs.length === 0) {
    return (
      <div
        role="status"
        className="rounded-lg border border-dashed border-gray-300 p-8 text-center text-gray-500"
      >
        No pipeline runs
      </div>
    );
  }

  // --- Success state ---
  return (
    <div className="space-y-4">
      {runs.map((run) => (
        <div
          key={run.id}
          data-testid="pipeline-run-card"
          className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm"
        >
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-900">{run.pipeline}</p>
              <p className="text-xs text-gray-500">{run.orchestrator}</p>
            </div>
            <StatusBadge status={run.status} />
          </div>

          <div className="mt-2 flex items-center gap-4 text-xs text-gray-500">
            <span>
              {run.phases.length} {run.phases.length === 1 ? 'phase' : 'phases'}
            </span>
            <span>Started {formatTime(run.started_at)}</span>
          </div>
        </div>
      ))}
    </div>
  );
}

// --- Sub-components ---

interface StatusBadgeProps {
  status: string;
}

function StatusBadge({ status }: StatusBadgeProps) {
  const colors: Record<string, string> = {
    completed: 'bg-green-100 text-green-800',
    failed: 'bg-red-100 text-red-800',
    running: 'bg-blue-100 text-blue-800',
    pending: 'bg-yellow-100 text-yellow-800',
  };

  const colorClass = colors[status] ?? 'bg-gray-100 text-gray-800';

  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${colorClass}`}
      aria-label={`status: ${status}`}
    >
      {status}
    </span>
  );
}

function formatTime(iso: string): string {
  const date = new Date(iso);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMin = Math.floor(diffMs / 60_000);

  if (diffMin < 1) return 'just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHours = Math.floor(diffMin / 60);
  if (diffHours < 24) return `${diffHours}h ago`;
  return date.toLocaleDateString();
}
