import { useQuery } from '@tanstack/react-query';
import { createRoute, redirect, useNavigate, useSearch } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';

// ─── Types ──────────────────────────────────────────────────────────────

type TicketSource = 'github' | 'jira' | 'linear';
type TicketStatus = 'open' | 'closed' | 'in_progress';

interface Relationship {
  type: string;
  target_id: string;
}

interface Ticket {
  id: string;
  project_id: string;
  external_id: string;
  source: TicketSource;
  title: string;
  description: string;
  status: TicketStatus;
  labels: string[];
  relationships: Relationship[];
  prs: string[];
  created_at: string;
  updated_at: string;
}

interface TicketPage {
  items: Ticket[];
  page: number;
  limit: number;
  total: number;
}

interface TicketsSearch {
  status?: TicketStatus;
  source?: TicketSource;
}

// ─── Route ──────────────────────────────────────────────────────────────

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/tickets',
  beforeLoad: ({ location }) => {
    const token = localStorage.getItem('flux_token');
    if (!token) {
      throw redirect({ to: '/login', search: { redirect: location.href } });
    }
  },
  component: TicketsPage,
  validateSearch: (search: Record<string, unknown>): TicketsSearch => ({
    status: (search.status as TicketStatus) || undefined,
    source: (search.source as TicketSource) || undefined,
  }),
});

// ─── Data fetching ──────────────────────────────────────────────────────

async function fetchTickets(params: TicketsSearch): Promise<TicketPage> {
  const searchParams = new URLSearchParams();
  if (params.status) searchParams.set('status', params.status);
  if (params.source) searchParams.set('source', params.source);

  const qs = searchParams.toString();
  const url = qs ? `/api/v1/tickets?${qs}` : '/api/v1/tickets';

  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = localStorage.getItem('flux_token');
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(url, { headers });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || res.statusText);
  }
  return res.json();
}

// ─── Component ──────────────────────────────────────────────────────────

const STATUS_OPTIONS: TicketStatus[] = ['open', 'closed', 'in_progress'];
const SOURCE_OPTIONS: TicketSource[] = ['github', 'jira', 'linear'];

function TicketsPage() {
  const search = useSearch({ from: Route.id });
  const navigate = useNavigate({ from: Route.id });

  const ticketsQuery = useQuery<TicketPage>({
    queryKey: ['tickets', search],
    queryFn: () => fetchTickets(search),
  });

  function setFilter(key: 'status' | 'source', value: string | undefined) {
    navigate({ search: (prev: TicketsSearch) => ({ ...prev, [key]: value || undefined }) });
  }

  // --- Loading ---
  if (ticketsQuery.isPending) {
    return (
      <div>
        <Header />
        <LoadingSkeleton />
      </div>
    );
  }

  // --- Error ---
  if (ticketsQuery.isError) {
    return (
      <div>
        <Header />
        <ErrorBanner
          message={extractErrorMessage(ticketsQuery.error)}
          onRetry={() => ticketsQuery.refetch()}
        />
      </div>
    );
  }

  const { items } = ticketsQuery.data;

  return (
    <div>
      <Header />
      <FilterBar
        status={search.status}
        source={search.source}
        onStatusChange={(v) => setFilter('status', v)}
        onSourceChange={(v) => setFilter('source', v)}
      />

      {!items?.length ? (
        <EmptyState />
      ) : (
        <div className="mt-4 space-y-3">
          {items.map((ticket) => (
            <TicketCard key={ticket.id} ticket={ticket} />
          ))}
        </div>
      )}
    </div>
  );
}

// ─── Sub-components ─────────────────────────────────────────────────────

function Header() {
  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900">Tickets</h1>
      <p className="mt-2 text-gray-600">View and manage tickets from connected sources.</p>
    </div>
  );
}

interface FilterBarProps {
  status: string | undefined;
  source: string | undefined;
  onStatusChange: (value: string | undefined) => void;
  onSourceChange: (value: string | undefined) => void;
}

function FilterBar({ status, source, onStatusChange, onSourceChange }: FilterBarProps) {
  return (
    <div className="mt-4 flex flex-wrap items-center gap-4">
      <label className="flex items-center gap-2 text-sm text-gray-700">
        Status
        <select
          className="rounded-md border border-gray-300 px-3 py-1.5 text-sm"
          value={status ?? ''}
          onChange={(e) => onStatusChange(e.target.value || undefined)}
        >
          <option value="">All</option>
          {STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      </label>

      <label className="flex items-center gap-2 text-sm text-gray-700">
        Source
        <select
          className="rounded-md border border-gray-300 px-3 py-1.5 text-sm"
          value={source ?? ''}
          onChange={(e) => onSourceChange(e.target.value || undefined)}
        >
          <option value="">All</option>
          {SOURCE_OPTIONS.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      </label>
    </div>
  );
}

interface TicketCardProps {
  ticket: Ticket;
}

function TicketCard({ ticket }: TicketCardProps) {
  const statusColor =
    ticket.status === 'open'
      ? 'bg-blue-100 text-blue-800'
      : ticket.status === 'in_progress'
        ? 'bg-yellow-100 text-yellow-800'
        : 'bg-gray-100 text-gray-600';

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="text-xs font-medium uppercase tracking-wider text-gray-500">
              {ticket.source}
            </span>
            <span
              className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${statusColor}`}
            >
              {ticket.status}
            </span>
            {ticket.external_id && (
              <span className="text-xs text-gray-400">{ticket.external_id}</span>
            )}
          </div>
          <h3 className="mt-1 text-sm font-medium text-gray-900">{ticket.title}</h3>

          {ticket.labels?.length > 0 && (
            <div className="mt-2 flex flex-wrap gap-1">
              {ticket.labels.map((label) => (
                <span
                  key={label}
                  className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600"
                >
                  {label}
                </span>
              ))}
            </div>
          )}
        </div>

        <div className="ml-4 flex shrink-0 items-start gap-3 text-xs text-gray-400">
          {ticket.relationships?.length > 0 && (
            <span>{ticket.relationships?.length} relationship{ticket.relationships?.length !== 1 ? 's' : ''}</span>
          )}
          {ticket.prs?.length > 0 && (
            <span>{ticket.prs?.length} PR{ticket.prs?.length !== 1 ? 's' : ''}</span>
          )}
        </div>
      </div>
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="mt-4 space-y-3" role="status" aria-label="loading">
      {[1, 2, 3].map((i) => (
        <div
          key={i}
          className="animate-pulse rounded-lg border border-gray-200 bg-white p-4"
        >
          <div className="h-3 w-16 rounded bg-gray-200" />
          <div className="mt-2 h-4 w-3/4 rounded bg-gray-200" />
          <div className="mt-2 h-3 w-1/4 rounded bg-gray-200" />
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
      className="mt-4 rounded-lg border border-red-200 bg-red-50 p-4"
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

function EmptyState() {
  return (
    <div
      role="status"
      className="mt-4 rounded-lg border border-dashed border-gray-300 p-8 text-center text-gray-500"
    >
      No tickets found
    </div>
  );
}

// ─── Helpers ────────────────────────────────────────────────────────────

function extractErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}
