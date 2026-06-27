import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { createRoute, redirect } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';
import { SyncStatusSection } from '../components/SyncStatusSection';

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: ({ location }) => {
    const token = localStorage.getItem('flux_token');
    if (!token) {
      throw redirect({ to: '/login', search: { redirect: location.href } });
    }
  },
  component: Dashboard,
});

/** Fetches a JSON array from the given URL. */
async function fetchArray(url: string): Promise<unknown[]> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(url, { headers });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }
  return res.json() as Promise<unknown[]>;
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
 * Dashboard page displaying live counts of projects, tickets, pull requests,
 * and pipeline runs fetched from the API.
 */
export function Dashboard() {
  const projectsQuery = useQuery<unknown[]>({
    queryKey: ['projects-count'],
    queryFn: () => fetchArray('/api/v1/projects'),
  });

  const ticketsQuery = useQuery<unknown[]>({
    queryKey: ['tickets-count'],
    queryFn: () => fetchArray('/api/v1/tickets'),
  });

  const pullRequestsQuery = useQuery<unknown[]>({
    queryKey: ['pull-requests-count'],
    queryFn: () => fetchArray('/api/v1/pull-requests'),
  });

  const pipelineRunsQuery = useQuery<unknown[]>({
    queryKey: ['pipeline-runs-count'],
    queryFn: () => fetchArray('/api/v1/pipeline-runs'),
  });

  const queryClient = useQueryClient();
  const syncMutation = useMutation({
    mutationFn: async () => {
      const token = getToken();
      const headers: Record<string, string> = { 'Content-Type': 'application/json' };
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const res = await fetch('/api/v1/sync/trigger', { method: 'POST', headers });
      if (!res.ok) throw new Error('Sync trigger failed');
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects-count'] });
      queryClient.invalidateQueries({ queryKey: ['tickets-count'] });
      queryClient.invalidateQueries({ queryKey: ['pull-requests-count'] });
      queryClient.invalidateQueries({ queryKey: ['pipeline-runs-count'] });
    },
  });

  // --- Loading state ---
  if (
    projectsQuery.isPending ||
    ticketsQuery.isPending ||
    pullRequestsQuery.isPending ||
    pipelineRunsQuery.isPending
  ) {
    return (
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
        <p className="mt-2 text-gray-600">
          Welcome to Flux — your control plane for agentic software development.
        </p>
        <div
          className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4"
          role="status"
          aria-label="loading"
        >
          {[1, 2, 3, 4].map((i) => (
            <div
              key={i}
              className="animate-pulse rounded-lg border border-gray-200 bg-white p-4 shadow-sm"
            >
              <div className="h-4 w-16 rounded bg-gray-200" />
              <div className="mt-2 h-8 w-12 rounded bg-gray-200" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  // --- Error state ---
  const error =
    projectsQuery.error ??
    ticketsQuery.error ??
    pullRequestsQuery.error ??
    pipelineRunsQuery.error;

  if (error) {
    const message = error instanceof Error ? error.message : String(error);
    return (
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
        <p className="mt-2 text-gray-600">
          Welcome to Flux — your control plane for agentic software development.
        </p>
        <div
          role="alert"
          className="mt-6 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800"
        >
          {message}
        </div>
      </div>
    );
  }

  // --- Success state ---
  const projectCount = (projectsQuery.data as unknown[])?.length ?? 0;
  const ticketCount = (ticketsQuery.data as { items?: unknown[] })?.items?.length ?? 0;
  const pullRequestCount = (pullRequestsQuery.data as { items?: unknown[] })?.items?.length ?? 0;
  const pipelineRunCount = (pipelineRunsQuery.data as { items?: unknown[] })?.items?.length ?? 0;

  return (
    <div>
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
          <p className="mt-2 text-gray-600">
            Welcome to Flux — your control plane for agentic software development.
          </p>
        </div>
        <button
          type="button"
          onClick={() => syncMutation.mutate()}
          disabled={syncMutation.isPending}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {syncMutation.isPending ? 'Syncing...' : 'Sync Now'}
        </button>
      </div>
      <div className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard title="Projects" count={projectCount} href="/projects" />
        <StatCard title="Tickets" count={ticketCount} href="/tickets" />
        <StatCard title="Pull Requests" count={pullRequestCount} href="/pull-requests" />
        <StatCard title="Pipeline Runs" count={pipelineRunCount} href="/pipeline-runs" />
      </div>
      <SyncStatusSection />
    </div>
  );
}

interface StatCardProps {
  /** Display title for the card. */
  title: string;
  /** Numeric count to display. */
  count: number;
  /** Link href for navigation. */
  href: string;
}

function StatCard({ title, count, href }: StatCardProps) {
  return (
    <a
      href={href}
      className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm transition hover:shadow-md"
    >
      <p className="text-sm text-gray-500">{title}</p>
      <p className="mt-1 text-3xl font-semibold text-gray-900">{count}</p>
    </a>
  );
}
