import { useQuery } from '@tanstack/react-query';

export interface PipelineRunDetailProps {
  runId: string;
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
 * Fetches a single pipeline run by ID.
 * GET /api/v1/pipeline-runs/{runId} → PipelineRun
 */
async function fetchPipelineRun(runId: string): Promise<PipelineRun> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(`/api/v1/pipeline-runs/${encodeURIComponent(runId)}`, {
    headers,
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }

  return res.json() as Promise<PipelineRun>;
}

/**
 * Formats nanoseconds into a human-readable duration string.
 */
function formatDuration(nanos: number): string {
  const ms = nanos / 1_000_000;
  if (ms < 1000) return `${Math.round(ms)}ms`;
  const secs = ms / 1000;
  if (secs < 60) return `${Math.round(secs)}s`;
  const mins = Math.floor(secs / 60);
  const remainingSecs = Math.round(secs % 60);
  return `${mins}m ${remainingSecs}s`;
}

/**
 * PipelineRunDetail fetches and displays a single pipeline run with its phases
 * and cost breakdown. Supports loading, error (server/network/404), and success states.
 */
export function PipelineRunDetail({ runId }: PipelineRunDetailProps) {
  const query = useQuery<PipelineRun>({
    queryKey: ['pipeline-run', runId],
    queryFn: () => fetchPipelineRun(runId),
  });

  // --- Loading state ---
  if (query.isPending) {
    return (
      <div
        role="status"
        aria-label="loading"
        className="flex items-center justify-center py-12"
      >
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-blue-200 border-t-blue-600" />
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

  const run = query.data;

  // --- Success state ---
  return (
    <div className="space-y-6">
      {/* Metadata */}
      <div className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-medium text-gray-900">{run.pipeline}</h2>
            <p className="text-sm text-gray-500">{run.orchestrator}</p>
          </div>
          <StatusBadge status={run.status} />
        </div>
        <p className="mt-2 text-sm text-gray-500">Started {formatTime(run.started_at)}</p>
      </div>

      {/* Phases */}
      <div className="space-y-3">
        <h3 className="text-sm font-medium uppercase tracking-wider text-gray-500">
          Phases
        </h3>
        {run.phases.map((phase) => (
          <PhaseCard key={phase.name} phase={phase} />
        ))}
      </div>

      {/* Cost breakdown */}
      {run.cost && (
        <div className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
          <h3 className="text-sm font-medium uppercase tracking-wider text-gray-500">
            Cost
          </h3>
          <p className="mt-1 text-lg font-medium text-gray-900">
            {run.cost.total.toFixed(2)} {run.cost.currency}
          </p>
          <div className="mt-2 space-y-1">
            {run.phases.map((phase) => {
              const phaseCost = run.cost?.by_phase[phase.name];
              return (
                phaseCost !== undefined && (
                  <div
                    key={phase.name}
                    className="flex justify-between text-sm text-gray-600"
                    aria-label={`${phase.name} cost`}
                  >
                    <span>{phaseCost.toFixed(2)}</span>
                  </div>
                )
              );
            })}
          </div>
        </div>
      )}
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

interface PhaseCardProps {
  phase: PipelineRunPhase;
}

function PhaseCard({ phase }: PhaseCardProps) {
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
      <div className="flex items-center justify-between">
        <p className="text-sm font-medium text-gray-900">{phase.name}</p>
        <div className="flex items-center gap-2">
          <PhaseStatusDot status={phase.status} />
          <span className="text-xs text-gray-500">{formatDuration(phase.duration)}</span>
        </div>
      </div>
      {phase.output && (
        <pre className="mt-2 rounded bg-green-50 p-2 text-xs text-green-800">
          {phase.output}
        </pre>
      )}
      {phase.error && (
        <pre className="mt-2 rounded bg-red-50 p-2 text-xs text-red-800">
          {phase.error}
        </pre>
      )}
    </div>
  );
}

interface PhaseStatusDotProps {
  status: string;
}

function PhaseStatusDot({ status }: PhaseStatusDotProps) {
  const colors: Record<string, string> = {
    completed: 'bg-green-500',
    failed: 'bg-red-500',
    running: 'bg-blue-500',
    pending: 'bg-yellow-500',
  };

  const colorClass = colors[status] ?? 'bg-gray-400';

  return (
    <span
      className={`inline-block h-2.5 w-2.5 rounded-full ${colorClass}`}
      aria-label={`status: ${status}`}
    />
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
