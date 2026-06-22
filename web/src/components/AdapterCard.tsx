export interface AdapterCardProps {
  type: string;
  name: string;
  health: 'healthy' | 'unhealthy' | 'unknown';
}

/**
 * AdapterCard displays a single adapter's type, display name, and health status.
 * Sync operations are handled at the list level (global sync).
 */
export function AdapterCard({ type, name, health }: AdapterCardProps) {
  const healthColor =
    health === 'healthy'
      ? 'bg-green-500'
      : health === 'unhealthy'
        ? 'bg-red-500'
        : 'bg-gray-400';

  return (
    <div
      data-testid="adapter-card"
      className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm"
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium uppercase tracking-wider text-gray-500">
            {type}
          </span>
          <span
            className={`inline-block h-2.5 w-2.5 rounded-full ${healthColor}`}
            aria-label={`Health: ${health}`}
          />
        </div>
        <p className="text-sm text-gray-900">{name}</p>
      </div>
    </div>
  );
}
