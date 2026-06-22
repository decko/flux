import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { createRoute, redirect } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';

// ─── Types ─────────────────────────────────────────────────────────────────

interface Review {
  author: string;
  status: string;
  comment: string;
  created_at: string;
}

interface PullRequest {
  id: string;
  project_id: string;
  external_id: string;
  source: string;
  title: string;
  url: string;
  status: 'open' | 'merged' | 'closed';
  ticket_ids: string[];
  reviews: Review[];
  created_at: string;
  updated_at: string;
}

interface PullRequestPage {
  items: PullRequest[];
}

// ─── Search params schema ─────────────────────────────────────────────────

interface PullRequestsSearch {
  status?: string;
}

// ─── Route definition ─────────────────────────────────────────────────────

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/pull-requests',
  beforeLoad: ({ location }) => {
    const token = localStorage.getItem('flux_token');
    if (!token) {
      throw redirect({ to: '/login', search: { redirect: location.href } });
    }
  },
  validateSearch: (search: Record<string, unknown>): PullRequestsSearch => ({
    status: typeof search.status === 'string' ? search.status : undefined,
  }),
  component: PullRequestsPage,
});

// ─── Status badge colors ──────────────────────────────────────────────────

const statusColors: Record<string, string> = {
  open: 'bg-green-100 text-green-800',
  merged: 'bg-purple-100 text-purple-800',
  closed: 'bg-red-100 text-red-800',
};

// ─── Pull Request Card ────────────────────────────────────────────────────

function PullRequestCard({ pr }: { pr: PullRequest }) {
  const ticketCount = pr.ticket_ids.length;
  const reviewCount = pr.reviews.length;

  return (
    <div
      data-testid="pr-card"
      className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm transition-shadow hover:shadow-md"
    >
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0 flex-1">
          <h3 className="text-base font-semibold text-gray-900">
            {pr.title}
          </h3>
          <div className="mt-1 flex items-center gap-3 text-sm text-gray-500">
            <span
              className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${statusColors[pr.status] ?? 'bg-gray-100 text-gray-800'}`}
            >
              {pr.status}
            </span>
            <span>
              {ticketCount} {ticketCount === 1 ? 'ticket' : 'tickets'}
            </span>
            <span>
              {reviewCount} {reviewCount === 1 ? 'review' : 'reviews'}
            </span>
          </div>
        </div>
        <a
          href={pr.url}
          target="_blank"
          rel="noopener noreferrer"
          className="shrink-0 text-sm text-blue-600 hover:text-blue-800 hover:underline"
          aria-label={`View on ${pr.source === 'github' ? 'GitHub' : 'GitLab'}`}
        >
          Open ↗
        </a>
      </div>
    </div>
  );
}

// ─── Loading Skeleton ─────────────────────────────────────────────────────

function LoadingSkeleton() {
  return (
    <div
      role="status"
      aria-label="Loading pull requests"
      className="space-y-4"
    >
      {[1, 2, 3].map((i) => (
        <div
          key={i}
          className="animate-pulse rounded-lg border border-gray-200 bg-white p-4"
        >
          <div className="h-5 w-3/4 rounded bg-gray-200" />
          <div className="mt-3 flex gap-3">
            <div className="h-5 w-16 rounded-full bg-gray-200" />
            <div className="h-5 w-20 rounded bg-gray-200" />
            <div className="h-5 w-20 rounded bg-gray-200" />
          </div>
        </div>
      ))}
    </div>
  );
}

// ─── Empty State ──────────────────────────────────────────────────────────

function EmptyState() {
  return (
    <div className="py-12 text-center">
      <p className="text-gray-500">No pull requests found.</p>
      <p className="mt-1 text-sm text-gray-400">
        Pull requests from connected repositories will appear here.
      </p>
    </div>
  );
}

// ─── Error Banner ─────────────────────────────────────────────────────────

function ErrorBanner({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  return (
    <div
      role="alert"
      className="rounded-lg border border-red-200 bg-red-50 p-4"
    >
      <div className="flex items-center justify-between">
        <p className="text-sm font-medium text-red-800">{message}</p>
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

// ─── Main Page Component ──────────────────────────────────────────────────

function PullRequestsPage() {
  const { status: initialStatus } = Route.useSearch();
  const [statusFilter, setStatusFilter] = useState<string>(initialStatus ?? '');

  const { data, isLoading, isError, error, refetch } = useQuery<PullRequestPage>({
    queryKey: ['pull-requests', statusFilter],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (statusFilter) {
        params.set('status', statusFilter);
      }
      const qs = params.toString();
      const url = `/api/v1/pull-requests${qs ? `?${qs}` : ''}`;
      const headers: Record<string, string> = {};
      const token = localStorage.getItem('flux_token');
      if (token) headers['Authorization'] = `Bearer ${token}`;

      const res = await fetch(url, { headers });
      if (!res.ok) {
        const body = (await res.json()) as { error?: string };
        throw new Error(body?.error ?? `Request failed with status ${res.status}`);
      }
      return res.json() as Promise<PullRequestPage>;
    },
  });

  const items = data?.items ?? [];

  // ── Loading state ────────────────────────────────────────────────────
  if (isLoading) {
    return (
      <div>
        <PageHeader />
        <FilterBar value={statusFilter} onChange={setStatusFilter} />
        <div className="mt-4">
          <LoadingSkeleton />
        </div>
      </div>
    );
  }

  // ── Error state ──────────────────────────────────────────────────────
  if (isError) {
    return (
      <div>
        <PageHeader />
        <FilterBar value={statusFilter} onChange={setStatusFilter} />
        <div className="mt-4">
          <ErrorBanner
            message={error?.message ?? 'Failed to load pull requests'}
            onRetry={() => refetch()}
          />
        </div>
      </div>
    );
  }

  // ── Empty state ──────────────────────────────────────────────────────
  if (items.length === 0) {
    return (
      <div>
        <PageHeader />
        <FilterBar value={statusFilter} onChange={setStatusFilter} />
        <EmptyState />
      </div>
    );
  }

  // ── Success state ────────────────────────────────────────────────────
  return (
    <div>
      <PageHeader />
      <FilterBar value={statusFilter} onChange={setStatusFilter} />
      <div className="mt-4 space-y-3">
        {items.map((pr) => (
          <PullRequestCard key={pr.id} pr={pr} />
        ))}
      </div>
    </div>
  );
}

// ─── Page Header ─────────────────────────────────────────────────────────

function PageHeader() {
  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900">Pull Requests</h1>
      <p className="mt-2 text-gray-600">
        Review open pull requests across repositories.
      </p>
    </div>
  );
}

// ─── Filter Bar ───────────────────────────────────────────────────────────

function FilterBar({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="mt-4 flex items-center gap-2">
      <label htmlFor="status-filter" className="text-sm font-medium text-gray-700">
        Status
      </label>
      <select
        id="status-filter"
        name="status"
        aria-label="Filter by status"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="rounded-md border border-gray-300 px-3 py-1.5 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
      >
        <option value="">All</option>
        <option value="open">Open</option>
        <option value="merged">Merged</option>
        <option value="closed">Closed</option>
      </select>
    </div>
  );
}
