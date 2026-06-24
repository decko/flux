import { useQuery } from '@tanstack/react-query';

/** Matches the backend model.AuditEvent JSON shape. */
interface AuditEvent {
  id: string;
  actor_id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  metadata: string;
  created_at: string;
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
 * Fetches audit events from the admin-only API.
 * GET /api/v1/audit-events → AuditEvent[]
 * Returns a discriminated result to distinguish 403 from other errors.
 */
async function fetchAuditEvents(): Promise<AuditEvent[]> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch('/api/v1/audit-events', { headers });

  if (res.status === 403) {
    throw new AccessDeniedError();
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }

  return res.json() as Promise<AuditEvent[]>;
}

/** Custom error class to distinguish 403 "Access denied" from other errors. */
export class AccessDeniedError extends Error {
  constructor() {
    super('Access denied');
    this.name = 'AccessDeniedError';
  }
}

/**
 * Formats an ISO-8601 timestamp into a locale-friendly string.
 * Falls back to the raw value if parsing fails.
 */
function formatTimestamp(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

/**
 * AuditLog displays a table of audit events fetched from the admin-only
 * GET /api/v1/audit-events endpoint.
 *
 * States:
 * - **Loading**: skeleton placeholders
 * - **Empty**: message when no events exist
 * - **Error**: banner with error message (includes "Access denied" for 403)
 * - **Success**: table with columns actor_id, action, resource_type, resource_id, created_at
 */
export function AuditLog() {
  const query = useQuery<AuditEvent[]>({
    queryKey: ['audit-events'],
    queryFn: fetchAuditEvents,
  });

  // --- Loading state ---
  if (query.isPending) {
    return (
      <div className="space-y-3" role="status" aria-label="loading">
        {[1, 2, 3].map((i) => (
          <div
            key={i}
            className="h-10 animate-pulse rounded bg-gray-200"
          />
        ))}
      </div>
    );
  }

  // --- Error state ---
  if (query.isError) {
    const isAccessDenied = query.error instanceof AccessDeniedError;

    if (isAccessDenied) {
      return (
        <div
          role="alert"
          className="rounded-lg border border-yellow-200 bg-yellow-50 p-4 text-sm text-yellow-800"
        >
          Access denied. Admin privileges are required to view audit events.
        </div>
      );
    }

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

  const events = query.data ?? [];

  // --- Empty state ---
  if (events.length === 0) {
    return (
      <div
        role="status"
        className="rounded-lg border border-dashed border-gray-300 p-8 text-center text-gray-500"
      >
        No audit events
      </div>
    );
  }

  // --- Success state ---
  return (
    <div className="overflow-x-auto rounded-lg border border-gray-200">
      <table className="min-w-full divide-y divide-gray-200 text-sm" role="table">
        <thead className="bg-gray-50">
          <tr>
            <th scope="col" className="px-4 py-3 text-left font-medium text-gray-500">
              Actor
            </th>
            <th scope="col" className="px-4 py-3 text-left font-medium text-gray-500">
              Action
            </th>
            <th scope="col" className="px-4 py-3 text-left font-medium text-gray-500">
              Resource Type
            </th>
            <th scope="col" className="px-4 py-3 text-left font-medium text-gray-500">
              Resource ID
            </th>
            <th scope="col" className="px-4 py-3 text-left font-medium text-gray-500">
              Created At
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100 bg-white">
          {events.map((event) => (
            <tr key={event.id} className="hover:bg-gray-50">
              <td className="whitespace-nowrap px-4 py-3 text-gray-900">
                {event.actor_id}
              </td>
              <td className="whitespace-nowrap px-4 py-3 text-gray-900">
                {event.action}
              </td>
              <td className="whitespace-nowrap px-4 py-3 text-gray-600">
                {event.resource_type}
              </td>
              <td className="whitespace-nowrap px-4 py-3 font-mono text-xs text-gray-600">
                {event.resource_id}
              </td>
              <td className="whitespace-nowrap px-4 py-3 text-gray-600">
                {formatTimestamp(event.created_at)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
